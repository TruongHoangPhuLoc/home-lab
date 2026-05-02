# incidents/

Cluster-wide incident reports. One file per incident, named
`YYYY-MM-DD-short-slug.md`. Each report is the post-mortem of a
real outage / data loss / partial-failure event in this homelab —
written to be useful to a future operator (you, in 18 months) who
hits the same shape of problem.

## Scope

Anything that:
- Caused user-visible degradation lasting more than ~10 minutes.
- Required out-of-band recovery (not just `kubectl rollout restart`).
- Resulted in data loss (accepted or not).
- Surfaced a class of mistake worth preventing structurally.

Smaller hiccups belong in the component-local `LESSONS.md` (e.g.,
[`platform/argocd/LESSONS.md`](../platform/argocd/LESSONS.md)) and
get promoted here only if the same root cause recurs.

## Format

Each report follows roughly:

```
# YYYY-MM-DD — short title

**Severity:** S1 / S2 / S3
**Duration:** ~Xh
**Window:** UTC start → UTC end

## Summary
1-paragraph plain-English description.

## Trigger
What command / change / event kicked it off.

## Detection
How and when we noticed.

## Root cause
The actual underlying cause, distinct from the trigger.

## Impact
Which components, which data, which users.

## Recovery timeline
Reverse-chronological or forward, with timestamps.

## What worked / What didn't
Honest assessment of the response.

## Action items
Concrete fixes, owners, status. Most important section — these
turn the incident into prevention.

## Lessons
Generalizable principles. Watch for recurrence. Promote into
[`../CLAUDE.md`](../CLAUDE.md) "Global rules" if the same
principle would have prevented multiple incidents.
```

## Prevention discipline

When you finish writing a report:
1. Pull every preventable cause into a concrete action item with an owner.
2. If a single rule would have stopped the incident, promote it into
   [`../CLAUDE.md`](../CLAUDE.md) "Destructive-operation rules".
3. If the rule is component-specific, promote it into the relevant
   `<dir>/CLAUDE.md`.
4. Don't let a report sit "for later" — the work happens now or not
   at all. The recovery already cost us; we get the prevention free
   only if we do it while the context is fresh.
