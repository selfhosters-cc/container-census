#!/usr/bin/env python3
"""
CPU Load Test Container
Oscillates between high and low CPU usage for testing Container Census stats
"""
import time
import math
import sys

def cpu_intensive_work(duration_seconds):
    """Do CPU-intensive work for specified duration"""
    end_time = time.time() + duration_seconds
    result = 0
    while time.time() < end_time:
        # CPU-intensive calculations
        for i in range(10000):
            result += math.sqrt(i) * math.sin(i) * math.cos(i)
    return result

def main():
    print("🔥 CPU Load Test Container Started")
    print("=" * 50)
    cycle = 0

    while True:
        cycle += 1

        # High CPU phase (5 seconds)
        print(f"\n[Cycle {cycle}] 🚀 HIGH CPU - Working hard...")
        sys.stdout.flush()
        cpu_intensive_work(5)

        # Low CPU phase (5 seconds)
        print(f"[Cycle {cycle}] 😴 LOW CPU - Resting...")
        sys.stdout.flush()
        time.sleep(5)

        # Idle phase (5 seconds)
        print(f"[Cycle {cycle}] 💤 IDLE - Sleeping...")
        sys.stdout.flush()
        time.sleep(5)

if __name__ == "__main__":
    main()
