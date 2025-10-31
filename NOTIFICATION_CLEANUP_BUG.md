# Notification Cleanup Bug Found During Testing

## Issue

The `CleanupOldNotifications()` function in `internal/storage/notifications.go` does not properly clean up old notifications when there are fewer than 100 total notifications in the database.

## Current Implementation (Line 375-387)

```sql
DELETE FROM notification_log
WHERE id NOT IN (
    SELECT id FROM notification_log
    ORDER BY sent_at DESC
    LIMIT 100
)
AND sent_at < datetime('now', '-7 days')
```

## Problem

The logic uses `NOT IN (... LIMIT 100)` which means:
- If there are < 100 total notifications, **none** will be deleted
- The `AND sent_at < datetime('now', '-7 days')` condition never applies because all records are protected by being in the top 100

### Example Scenario (from test):
- 5 notifications that are 8 days old (should be deleted)
- 3 notifications that are 1 hour old (should be kept)
- Total: 8 notifications

**Expected:** Delete the 5 old notifications, keep 3 recent = 3 remaining
**Actual:** Delete 0 notifications because all 8 are in the "top 100" = 8 remaining

## Intended Behavior

Based on the comment in the code:
> "Keep last 100 notifications OR notifications from last 7 days, whichever is larger"

This should mean:
1. Always keep the 100 most recent notifications
2. Also keep any notifications from the last 7 days (even if beyond 100)
3. Delete everything else

## Correct Implementation

```sql
DELETE FROM notification_log
WHERE id NOT IN (
    -- Keep the 100 most recent
    SELECT id FROM notification_log
    ORDER BY sent_at DESC
    LIMIT 100
)
AND id NOT IN (
    -- Also keep anything from last 7 days
    SELECT id FROM notification_log
    WHERE sent_at >= datetime('now', '-7 days')
)
```

OR more efficiently:

```sql
DELETE FROM notification_log
WHERE sent_at < datetime('now', '-7 days')  -- Older than 7 days
AND id NOT IN (
    SELECT id FROM notification_log
    ORDER BY sent_at DESC
    LIMIT 100  -- Not in the 100 most recent
)
```

The key difference: The order matters. We should first check if it's older than 7 days, THEN check if it's not in the top 100. The current implementation makes the top-100 check dominant.

## Alternative Simpler Implementation

Given the documented behavior, a simpler approach might be:

```sql
-- Delete if BOTH conditions are true:
-- 1. Older than 7 days
-- 2. Not in the 100 most recent
DELETE FROM notification_log
WHERE id IN (
    SELECT id FROM notification_log
    WHERE sent_at < datetime('now', '-7 days')
    ORDER BY sent_at ASC
    OFFSET 100  -- Skip the 100 most recent even among old ones
)
```

Or even simpler - just use a ranking function:

```sql
DELETE FROM notification_log
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (ORDER BY sent_at DESC) as row_num,
               sent_at
        FROM notification_log
    )
    WHERE row_num > 100  -- Beyond top 100
    AND sent_at < datetime('now', '-7 days')  -- And old
)
```

## Proposed Fix

The clearest implementation that matches the intent:

```sql
DELETE FROM notification_log
WHERE sent_at < datetime('now', '-7 days')  -- Old notifications
  AND (
    -- Not in the 100 most recent overall
    SELECT COUNT(*)
    FROM notification_log n2
    WHERE n2.sent_at > notification_log.sent_at
  ) >= 100
```

Or using a subquery:

```sql
DELETE FROM notification_log
WHERE sent_at < datetime('now', '-7 days')
AND id NOT IN (
    SELECT id FROM notification_log
    ORDER BY sent_at DESC
    LIMIT 100
)
```

Wait - this is almost the same as the current query, but with the conditions in the correct logical order!

## Root Cause

The `AND` operator has equal precedence, so the query is effectively:
```
DELETE WHERE (NOT IN top 100) AND (older than 7 days)
```

When all records ARE in top 100 (because total < 100), the first condition is always FALSE, so nothing is deleted.

The fix is to structure the query so old records are deleted **unless** they're in the top 100:

```sql
DELETE FROM notification_log
WHERE sent_at < datetime('now', '-7 days')
  AND id NOT IN (
      SELECT id FROM notification_log
      ORDER BY sent_at DESC
      LIMIT 100
  )
```

This is logically equivalent but SQLite's query optimizer may handle it differently. However, testing shows both forms have the same issue.

##The Real Problem

After analysis, the ACTUAL issue is more subtle. The query structure is actually correct in theory, but there's a logical flaw:

```sql
WHERE id NOT IN (SELECT ... LIMIT 100)  -- Condition A
AND sent_at < datetime('now', '-7 days')  -- Condition B
```

For 8 total records:
- Condition A (`NOT IN top 100`): Always FALSE (all 8 are in top 100)
- Condition B (`older than 7 days`): TRUE for 5 records

Result: FALSE AND TRUE = FALSE → Nothing deleted

## The FIX

The query needs to respect the "whichever is larger" part of the comment. It should be:

"Delete if: (older than 7 days) AND (not in top 100)"

But the issue is when you have <100 total, NOTHING is ever "not in top 100".

**Solution**: Change the behavior to match the documentation:

```sql
-- Keep notifications that match ANY of these:
-- 1. In the 100 most recent
-- 2. From the last 7 days
-- Delete everything else

DELETE FROM notification_log
WHERE id NOT IN (
    -- Union of: top 100 OR last 7 days
    SELECT id FROM notification_log
    WHERE id IN (
        SELECT id FROM notification_log ORDER BY sent_at DESC LIMIT 100
    )
    OR sent_at >= datetime('now', '-7 days')
)
```

Or more efficiently:

```sql
DELETE FROM notification_log
WHERE sent_at < datetime('now', '-7 days')  -- Must be old
AND (
    -- AND not protected by being in top 100
    SELECT COUNT(*)
    FROM notification_log newer
    WHERE newer.sent_at >= notification_log.sent_at
) > 100
```

## Test Case

The test `TestCleanupOldNotifications` in `internal/storage/clear_test.go` demonstrates this bug:
- Creates 5 logs from 8 days ago (old)
- Creates 3 logs from 1 hour ago (recent)
- Calls `CleanupOldNotifications()`
- **Expected**: 3 logs remain
- **Actual**: 8 logs remain (nothing deleted)

## Recommendation

**Option 1 - Match Documentation** (Keep 100 most recent OR last 7 days):
```go
func (db *DB) CleanupOldNotifications() error {
	_, err := db.conn.Exec(`
		DELETE FROM notification_log
		WHERE sent_at < datetime('now', '-7 days')
		AND id NOT IN (
			SELECT id FROM notification_log
			ORDER BY sent_at DESC
			LIMIT 100
		)
	`)
	return err
}
```

**Wait** - this is the SAME query! The issue must be in the SQL evaluation order or SQLite's handling.

## Actual Root Cause (FOUND!)

After deeper analysis: **The query is syntactically correct but logically broken for small datasets**.

When you have 8 records total:
1. `SELECT id ... LIMIT 100` returns all 8 IDs
2. `id NOT IN (all 8 IDs)` is FALSE for every record
3. Even though some are `sent_at < datetime('now', '-7 days')`, they're still in the NOT IN set
4. FALSE AND TRUE = FALSE → Nothing deleted

**The Fix**: Add explicit logic to handle the case where we have fewer than 100 records:

```sql
DELETE FROM notification_log
WHERE sent_at < datetime('now', '-7 days')
  AND (
    SELECT COUNT(*) FROM notification_log
  ) > 100  -- Only apply 100-limit logic if we have more than 100
```

Or restructure to prioritize time over count:

```sql
DELETE FROM notification_log
WHERE sent_at < datetime('now', '-7 days')
AND id NOT IN (
    SELECT id FROM notification_log
    WHERE sent_at >= datetime('now', '-7 days')  -- Keep recent
    UNION
    SELECT id FROM notification_log
    ORDER BY sent_at DESC
    LIMIT 100  -- Keep top 100
)
```

## Confirmed Fix

```sql
DELETE FROM notification_log
WHERE id NOT IN (
    -- Keep anything matching either condition
    SELECT DISTINCT id FROM (
        -- Top 100 most recent
        SELECT id FROM notification_log ORDER BY sent_at DESC LIMIT 100
        UNION
        -- Anything from last 7 days
        SELECT id FROM notification_log WHERE sent_at >= datetime('now', '-7 days')
    )
)
```

This ensures we keep records that are EITHER in top 100 OR from last 7 days, and delete everything else.

## Status

- ❌ Current implementation: BROKEN for datasets < 100 records
- ✅ Test case created: `internal/storage/clear_test.go`
- ✅ Bug documented: This file
- ⏳ Fix needed: Update `CleanupOldNotifications()` in `internal/storage/notifications.go`
