# Idle Cron — Run a Task When Claude Is Idle

Run a task repeatedly at a configurable interval, but ONLY when Claude
has nothing else going on — no agents running, no tools executing, no
pending work. If Claude is busy with something else, skip and check
again next interval.

## Arguments

$ARGUMENTS: `<interval> <task description>`

Examples:
- `/cron 30m review code for improvements`
- `/cron 1h run all tests and fix failures`
- `/cron 15m check CI status and fix broken builds`
- `/cron 2h find and fix security issues`

If no arguments, ask the user what to run and how often.

## How It Works

### 1. Parse arguments

- Interval: first argument (e.g., `30m`, `1h`, `15m`, `2h`)
  - `m` = minutes, `h` = hours
  - Default: 30m
- Task: everything after the interval

### 2. Idle detection

"Idle" means Claude is NOT currently:
- Running any background agents (Agent tool with run_in_background)
- Executing any tool calls
- Waiting for user input on a permission prompt
- In the middle of generating a response

The cron task should ONLY run when the user is not actively
interacting with Claude and no background work is happening.

Implementation: Use a simple sleep loop. After completing any user
request, if the cron is active, start the timer. If the user sends
a new message before the timer fires, reset the timer. The task
only executes after the full interval passes with no activity.

```
after_each_response:
    if cron_active and no_pending_agents:
        sleep(interval)
        if still_idle:  # no new messages arrived during sleep
            run_task()
        else:
            reset_timer()
```

### 3. Execute the task

When idle, execute the user's task description. The task runs as a
sub-agent so the main conversation stays clean:

- Launch an Agent with the task description
- The agent works independently (reads code, makes changes, runs tests)
- When done, report a summary back to the main conversation
- If changes were made, create a PR (never push to main)

### 4. After task completion

- Log: timestamp, what was done, outcome
- If changes: create PR with description of what was improved
- Reset timer for next interval
- If task found nothing to do: log "no action needed" and wait

## Example Session

```
User: /cron 30m improve the codebase

Claude: Idle cron started.
  Task: improve the codebase
  Interval: every 30 minutes
  Will run when Claude has been idle for 30 minutes.

  ... user works normally, cron timer resets on each interaction ...

  [14:30] Idle for 30m. Running task...
  [14:31] Found: test_archiver.py has no test for flush timeout.
          Added test_flush_timeout_triggers_flush. Tests pass.
  [14:31] Created PR #22: Add flush timeout test for archiver
  [14:31] Next run after 30m of idle time.

  ... user comes back, works on something else ...
  ... timer resets ...

  [16:00] Idle for 30m. Running task...
  [16:00] No improvements found. All tests pass. Code looks good.
  [16:00] Next run after 30m of idle time.
```

## Common Tasks for EntDB

- `improve test coverage` — find untested code paths, add tests
- `find and fix bugs` — critical code review + fixes
- `optimize performance` — find bottlenecks, improve hot paths
- `update documentation` — sync docs with current code
- `fix CI issues` — check workflow status, fix failures
- `security review` — find vulnerabilities, fix them
- `refactor for readability` — simplify complex functions

## Important

- Cron runs until you stop it (Ctrl+C or `/cron stop`)
- Timer resets on ANY user interaction — never interrupts active work
- Each task runs as an independent agent — isolated from main conversation
- Changes always go through PRs, never direct to main
- If the task errors, log it and continue to next interval
- Multiple crons are not supported — starting a new one replaces the old one
