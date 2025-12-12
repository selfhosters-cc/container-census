import { useEffect, useState } from 'react';

interface ProgressData {
  total: number;
  checked: number;
  status: string;
}

interface CompleteData {
  results: Record<string, any>;
  status: string;
  error?: string;
}

interface UseUpdateCheckProgressReturn {
  progress: ProgressData | null;
  complete: CompleteData | null;
  error: string | null;
}

/**
 * Custom hook to track progress of a bulk container update check via Server-Sent Events
 * @param jobId - The ID of the update check job to track
 * @returns Progress data, completion data, and any errors
 */
export function useUpdateCheckProgress(jobId: string | null): UseUpdateCheckProgressReturn {
  const [progress, setProgress] = useState<ProgressData | null>(null);
  const [complete, setComplete] = useState<CompleteData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!jobId) {
      // Reset state when jobId is cleared
      setProgress(null);
      setComplete(null);
      setError(null);
      return;
    }

    // Check if browser supports Server-Sent Events
    if (typeof EventSource === 'undefined') {
      setError('Your browser does not support real-time updates');
      return;
    }

    const eventSource = new EventSource(`/api/containers/check-progress/${jobId}`);

    eventSource.addEventListener('progress', (event) => {
      try {
        const data = JSON.parse(event.data) as ProgressData;
        setProgress(data);
      } catch (err) {
        console.error('Failed to parse progress data:', err);
      }
    });

    eventSource.addEventListener('complete', (event) => {
      try {
        const data = JSON.parse(event.data) as CompleteData;
        setComplete(data);
        eventSource.close();
      } catch (err) {
        console.error('Failed to parse complete data:', err);
        setError('Failed to parse completion data');
        eventSource.close();
      }
    });

    eventSource.addEventListener('error', (event: any) => {
      try {
        if (event.data) {
          const errorData = JSON.parse(event.data);
          setError(errorData.error || 'Error checking for updates');
        } else {
          setError('Connection error while checking for updates');
        }
      } catch (err) {
        setError('Error checking for updates');
      }
      eventSource.close();
    });

    eventSource.onerror = (err) => {
      console.error('SSE error:', err);
      if (eventSource.readyState === EventSource.CLOSED) {
        setError('Connection closed unexpectedly');
      }
      eventSource.close();
    };

    // Cleanup function
    return () => {
      eventSource.close();
    };
  }, [jobId]);

  return { progress, complete, error };
}
