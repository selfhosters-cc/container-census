/**
 * Container Census - Graph Visualizer Plugin
 * Entry point for the plugin frontend
 */

import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import './styles.css';

// Register dagre layout
cytoscape.use(dagre);

/**
 * Initialize the graph visualizer plugin
 * This function is called by the Census Plugin Loader
 *
 * @param {HTMLElement} container - The container element to render into
 * @param {CensusPluginSDK} sdk - The Census Plugin SDK instance
 */
export function initGraphVisualizer(container, sdk) {
  const graphView = new GraphVisualizerView(container, sdk);
  graphView.init();
}

/**
 * Main view class for the graph visualizer
 */
class GraphVisualizerView {
  constructor(container, sdk) {
    this.container = container;
    this.sdk = sdk;
    this.cy = null;
    this.currentLayout = 'cose';  // Use force-directed as default for better clustering
    this.colorMode = 'status';  // 'status' or 'project'
    this.showNetworks = true;
    this.showVolumes = true;
    this.showStoppedContainers = true;
    this.projectColors = {}; // Cache for project color assignments
  }

  /**
   * Initialize the view
   */
  async init() {
    this.render();
    await this.loadGraph();
  }

  /**
   * Render the UI structure
   */
  render() {
    this.container.innerHTML = `
      <div class="graph-visualizer">
        <div class="graph-toolbar">
          <div class="graph-toolbar-left">
            <h3 style="margin: 0; font-size: 16px;">🕸️ Container Relationship Graph</h3>
          </div>
          <div class="graph-toolbar-right">
            <label class="graph-filter">
              <input type="checkbox" id="showNetworks" checked>
              <span>Networks</span>
            </label>
            <label class="graph-filter">
              <input type="checkbox" id="showVolumes" checked>
              <span>Volumes</span>
            </label>
            <label class="graph-filter">
              <input type="checkbox" id="showStoppedContainers" checked>
              <span>Stopped</span>
            </label>
            <select id="colorModeSelect" class="graph-layout-select">
              <option value="status" selected>Color by Status</option>
              <option value="project">Color by Project</option>
            </select>
            <select id="layoutSelect" class="graph-layout-select">
              <option value="cose" selected>Force-Directed</option>
              <option value="dagre">Dagre (Hierarchical)</option>
              <option value="circle">Circle</option>
              <option value="grid">Grid</option>
            </select>
            <button id="zoomOutBtn" class="graph-btn graph-btn-secondary">
              ➖ Zoom Out
            </button>
            <button id="zoomInBtn" class="graph-btn graph-btn-secondary">
              ➕ Zoom In
            </button>
            <button id="refreshBtn" class="graph-btn graph-btn-primary">
              🔄 Refresh
            </button>
            <button id="fitBtn" class="graph-btn graph-btn-secondary">
              🎯 Fit to View
            </button>
          </div>
        </div>
        <div class="graph-container" id="graphContainer"></div>
        <div class="graph-legend" id="graphLegend">
          <!-- Legend items will be dynamically rendered based on color mode -->
        </div>
      </div>
    `;

    // Set up event listeners
    this.setupEventListeners();
  }

  /**
   * Set up event listeners for controls
   */
  setupEventListeners() {
    const refreshBtn = document.getElementById('refreshBtn');
    const fitBtn = document.getElementById('fitBtn');
    const zoomInBtn = document.getElementById('zoomInBtn');
    const zoomOutBtn = document.getElementById('zoomOutBtn');
    const layoutSelect = document.getElementById('layoutSelect');
    const colorModeSelect = document.getElementById('colorModeSelect');
    const showNetworks = document.getElementById('showNetworks');
    const showVolumes = document.getElementById('showVolumes');
    const showStoppedContainers = document.getElementById('showStoppedContainers');

    refreshBtn.addEventListener('click', () => this.loadGraph());
    fitBtn.addEventListener('click', () => this.fitGraph());
    zoomInBtn.addEventListener('click', () => this.zoomIn());
    zoomOutBtn.addEventListener('click', () => this.zoomOut());

    layoutSelect.addEventListener('change', (e) => {
      this.currentLayout = e.target.value;
      this.applyLayout();
    });

    colorModeSelect.addEventListener('change', (e) => {
      this.colorMode = e.target.value;
      this.updateColors();
      this.updateLegend();
    });

    showNetworks.addEventListener('change', (e) => {
      this.showNetworks = e.target.checked;
      this.applyFilters();
    });

    showVolumes.addEventListener('change', (e) => {
      this.showVolumes = e.target.checked;
      this.applyFilters();
    });

    showStoppedContainers.addEventListener('change', (e) => {
      this.showStoppedContainers = e.target.checked;
      this.applyFilters();
    });
  }

  /**
   * Load graph data from the plugin backend
   */
  async loadGraph() {
    try {
      this.sdk.showToast('Loading graph data...', 'info');

      const response = await this.sdk.fetch('/graph-data');

      if (!response.ok) {
        throw new Error(`Failed to load graph: ${response.statusText}`);
      }

      const data = await response.json();

      this.renderGraph(data);
      this.sdk.showToast('Graph loaded successfully!', 'success');
    } catch (error) {
      console.error('Failed to load graph:', error);
      this.sdk.showToast(`Failed to load graph: ${error.message}`, 'error');
      this.renderError(error.message);
    }
  }

  /**
   * Render the graph using Cytoscape
   */
  renderGraph(data) {
    const graphContainer = document.getElementById('graphContainer');

    // Clear existing graph
    if (this.cy) {
      this.cy.destroy();
    }

    // Convert data to Cytoscape format
    const elements = this.convertToElements(data);

    // Initialize Cytoscape with much faster zoom (20x more responsive)
    this.cy = cytoscape({
      container: graphContainer,
      elements: elements,
      style: this.getStylesheet(),
      layout: this.getLayoutConfig(),
      minZoom: 0.01,
      maxZoom: 20,
      wheelSensitivity: 0.05,  // 20x faster zoom than default (1.0)
    });

    // Set up node click handler
    this.cy.on('tap', 'node', (evt) => {
      const node = evt.target;
      this.showNodeDetails(node);
    });

    // Apply initial filters
    this.applyFilters();

    // Update legend based on current color mode
    this.updateLegend();
  }

  /**
   * Convert graph data to Cytoscape elements format
   */
  convertToElements(data) {
    const nodes = data.nodes.map(n => ({
      data: {
        id: n.id,
        label: n.label,
        type: n.type,
        status: n.status,
        host_id: n.host_id,
      },
    }));

    const edges = data.edges.map(e => ({
      data: {
        id: `${e.from}-${e.to}-${e.type}`,
        source: e.from,
        target: e.to,
        type: e.type,
        label: e.label || '',
      },
    }));

    return [...nodes, ...edges];
  }

  /**
   * Get Cytoscape stylesheet
   */
  getStylesheet() {
    return [
      // Container nodes
      {
        selector: 'node[type="container"]',
        style: {
          'background-color': (ele) => this.getContainerColor(ele),
          'label': 'data(label)',
          'color': '#ffffff',
          'text-valign': 'center',
          'text-halign': 'center',
          'font-size': '12px',
          'font-weight': '500',
          'width': 60,
          'height': 60,
          'shape': 'roundrectangle',
          'border-width': 2,
          'border-color': '#ffffff',
          'text-wrap': 'wrap',
          'text-max-width': '60px',
        },
      },
      // Network nodes
      {
        selector: 'node[type="network"]',
        style: {
          'background-color': '#3b82f6',
          'label': 'data(label)',
          'color': '#ffffff',
          'text-valign': 'center',
          'text-halign': 'center',
          'font-size': '11px',
          'width': 50,
          'height': 50,
          'shape': 'ellipse',
          'border-width': 2,
          'border-color': '#ffffff',
        },
      },
      // Volume nodes
      {
        selector: 'node[type="volume"]',
        style: {
          'background-color': '#f59e0b',
          'label': 'data(label)',
          'color': '#ffffff',
          'text-valign': 'center',
          'text-halign': 'center',
          'font-size': '11px',
          'width': 50,
          'height': 50,
          'shape': 'barrel',
          'border-width': 2,
          'border-color': '#ffffff',
        },
      },
      // Edges
      {
        selector: 'edge',
        style: {
          'width': 2,
          'line-color': '#cbd5e1',
          'target-arrow-color': '#cbd5e1',
          'target-arrow-shape': 'triangle',
          'curve-style': 'bezier',
          'opacity': 0.6,
        },
      },
      // Edge types
      {
        selector: 'edge[type="network"]',
        style: {
          'line-color': '#3b82f6',
          'target-arrow-color': '#3b82f6',
        },
      },
      {
        selector: 'edge[type="volume"]',
        style: {
          'line-color': '#f59e0b',
          'target-arrow-color': '#f59e0b',
        },
      },
      // Selected node
      {
        selector: 'node:selected',
        style: {
          'border-width': 4,
          'border-color': '#ef4444',
        },
      },
      // Hidden elements
      {
        selector: '.hidden',
        style: {
          'display': 'none',
        },
      },
    ];
  }

  /**
   * Get layout configuration
   */
  getLayoutConfig() {
    const configs = {
      dagre: {
        name: 'dagre',
        rankDir: 'LR',  // Left-to-right for better horizontal use
        nodeSep: 50,     // Tighter spacing
        rankSep: 150,    // More space between ranks
        animate: true,
        animationDuration: 500,
      },
      circle: {
        name: 'circle',
        animate: true,
        animationDuration: 500,
      },
      grid: {
        name: 'grid',
        animate: true,
        animationDuration: 500,
      },
      cose: {
        name: 'cose',
        animate: true,
        animationDuration: 500,
        nodeRepulsion: 4000,      // Less repulsion for tighter clusters
        idealEdgeLength: 80,      // Shorter edges
        gravity: 1,               // Pull nodes toward center
        numIter: 1000,            // More iterations for better layout
        componentSpacing: 100,    // Space between disconnected components
      },
    };

    return configs[this.currentLayout] || configs.cose;  // Use cose as default for better clustering
  }

  /**
   * Apply current layout
   */
  applyLayout() {
    if (!this.cy) return;

    const layout = this.cy.layout(this.getLayoutConfig());
    layout.run();
  }

  /**
   * Apply visibility filters
   */
  applyFilters() {
    if (!this.cy) return;

    // Show all elements first
    this.cy.elements().removeClass('hidden');

    // Hide networks if disabled
    if (!this.showNetworks) {
      this.cy.nodes('[type="network"]').addClass('hidden');
      this.cy.edges('[type="network"]').addClass('hidden');
    }

    // Hide volumes if disabled
    if (!this.showVolumes) {
      this.cy.nodes('[type="volume"]').addClass('hidden');
      this.cy.edges('[type="volume"]').addClass('hidden');
    }

    // Hide stopped containers if disabled
    if (!this.showStoppedContainers) {
      this.cy.nodes('[type="container"]').filter((node) => {
        const status = node.data('status');
        // Hide containers that are NOT running (status doesn't start with "Up")
        const isRunning = status && status.toLowerCase().startsWith('up');
        return !isRunning;
      }).addClass('hidden');
    }
  }

  /**
   * Fit graph to view
   */
  fitGraph() {
    if (!this.cy) return;
    this.cy.fit(null, 50);
  }

  /**
   * Zoom in by 50%
   */
  zoomIn() {
    if (!this.cy) return;
    const currentZoom = this.cy.zoom();
    this.cy.zoom({
      level: currentZoom * 1.5,  // 50% increase
      renderedPosition: { x: this.cy.width() / 2, y: this.cy.height() / 2 }
    });
  }

  /**
   * Zoom out by 33%
   */
  zoomOut() {
    if (!this.cy) return;
    const currentZoom = this.cy.zoom();
    this.cy.zoom({
      level: currentZoom / 1.5,  // 33% decrease (inverse of 1.5x)
      renderedPosition: { x: this.cy.width() / 2, y: this.cy.height() / 2 }
    });
  }

  /**
   * Get color for a container based on current color mode
   */
  getContainerColor(ele) {
    if (this.colorMode === 'status') {
      const status = ele.data('status');
      // Status field contains strings like "Up 2 days" or "Exited (0) 3 hours ago"
      const isRunning = status && status.toLowerCase().startsWith('up');
      return isRunning ? '#10b981' : '#ef4444';
    } else if (this.colorMode === 'project') {
      const project = ele.data('compose_project');
      if (!project) return '#6b7280'; // Gray for containers without a project
      return this.getProjectColor(project);
    }
    return '#6b7280';
  }

  /**
   * Get a consistent color for a project name
   */
  getProjectColor(project) {
    if (this.projectColors[project]) {
      return this.projectColors[project];
    }

    // Generate distinct colors for different projects
    const colors = [
      '#3b82f6', // Blue
      '#10b981', // Green
      '#f59e0b', // Amber
      '#ef4444', // Red
      '#8b5cf6', // Purple
      '#ec4899', // Pink
      '#14b8a6', // Teal
      '#f97316', // Orange
      '#06b6d4', // Cyan
      '#84cc16', // Lime
    ];

    const index = Object.keys(this.projectColors).length % colors.length;
    this.projectColors[project] = colors[index];
    return colors[index];
  }

  /**
   * Update colors when color mode changes
   */
  updateColors() {
    if (!this.cy) return;

    // Clear and rebuild project colors cache
    this.projectColors = {};

    // Force style refresh by updating stylesheet
    this.cy.style(this.getStylesheet());
  }

  /**
   * Update legend based on current color mode
   */
  updateLegend() {
    const legendContainer = document.getElementById('graphLegend');

    if (this.colorMode === 'status') {
      legendContainer.innerHTML = `
        <div class="legend-item">
          <span class="legend-box" style="background: #10b981;"></span>
          <span>Running</span>
        </div>
        <div class="legend-item">
          <span class="legend-box" style="background: #ef4444;"></span>
          <span>Stopped</span>
        </div>
        <div class="legend-item">
          <span class="legend-box" style="background: #3b82f6;"></span>
          <span>Network</span>
        </div>
        <div class="legend-item">
          <span class="legend-box" style="background: #f59e0b;"></span>
          <span>Volume</span>
        </div>
      `;
    } else if (this.colorMode === 'project') {
      let items = `
        <div class="legend-item">
          <span class="legend-box" style="background: #6b7280;"></span>
          <span>No Project</span>
        </div>
      `;

      // Add legend items for each project
      const projects = Object.keys(this.projectColors).sort();
      for (const project of projects) {
        items += `
          <div class="legend-item">
            <span class="legend-box" style="background: ${this.projectColors[project]};"></span>
            <span>${project}</span>
          </div>
        `;
      }

      items += `
        <div class="legend-item">
          <span class="legend-box" style="background: #3b82f6;"></span>
          <span>Network</span>
        </div>
        <div class="legend-item">
          <span class="legend-box" style="background: #f59e0b;"></span>
          <span>Volume</span>
        </div>
      `;

      legendContainer.innerHTML = items;
    }
  }

  /**
   * Show details for a node
   */
  showNodeDetails(node) {
    const data = node.data();

    let details = `<strong>Type:</strong> ${data.type}<br>`;
    details += `<strong>Label:</strong> ${data.label}<br>`;

    if (data.type === 'container') {
      details += `<strong>Status:</strong> ${data.status}<br>`;
      details += `<strong>Host ID:</strong> ${data.host_id}<br>`;
    }

    // Show toast with node details
    this.sdk.showToast(details, 'info');
  }

  /**
   * Render error message
   */
  renderError(message) {
    const graphContainer = document.getElementById('graphContainer');
    graphContainer.innerHTML = `
      <div style="display: flex; align-items: center; justify-content: center; height: 100%; color: #ef4444;">
        <div style="text-align: center;">
          <p style="font-size: 18px; margin-bottom: 10px;">⚠️ Error Loading Graph</p>
          <p style="font-size: 14px; color: #6b7280;">${message}</p>
          <button onclick="window.location.reload()" style="margin-top: 20px; padding: 8px 16px; background: #3b82f6; color: white; border: none; border-radius: 4px; cursor: pointer;">
            Reload Page
          </button>
        </div>
      </div>
    `;
  }
}

// Expose to global scope for Census Plugin Loader
if (typeof window !== 'undefined') {
  window.initGraphVisualizer = initGraphVisualizer;
}
