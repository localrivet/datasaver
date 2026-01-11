# Ralph Wiggum Guide for Claude

How to use the Ralph Wiggum/Ralph Loop skill in Claude Code.

## What is Ralph Loop?

Ralph Loop implements an iterative development methodology where Claude receives the same prompt repeatedly, seeing its previous work in files each iteration. This creates a self-improving loop until the task is complete.

## How to Invoke

Use the Skill tool with `ralph-wiggum:ralph-loop`:

```
Skill tool parameters:
  skill: "ralph-wiggum:ralph-loop"
  args: "Your task description" --max-iterations 20 --completion-promise "DONE"
```

### Key Points

1. **Quote the prompt** - The task description must be in quotes
2. **Put options after the prompt** - `--max-iterations` and `--completion-promise` come after
3. **Avoid angle brackets in args** - Don't put `<promise>` tags in the args, they get interpreted as shell redirects

### Options

- `--max-iterations N` - Stop after N iterations (prevents infinite loops)
- `--completion-promise "TEXT"` - The phrase that signals completion

## Examples

### Basic usage
```
skill: "ralph-wiggum:ralph-loop"
args: "Build a REST API" --max-iterations 15 --completion-promise "API COMPLETE"
```

### Complex task
```
skill: "ralph-wiggum:ralph-loop"
args: "Refactor the authentication system to use JWT tokens" --max-iterations 25 --completion-promise "REFACTOR DONE"
```

## How to Complete the Loop

When your task is genuinely finished, output the completion promise in XML tags:

```
<promise>YOUR COMPLETION TEXT HERE</promise>
```

For example, if `--completion-promise "API COMPLETE"` was set:
```
<promise>API COMPLETE</promise>
```

**CRITICAL**: Only output the promise when it's TRUE. Don't lie to exit the loop.

## How to Cancel

Use the cancel skill:
```
skill: "ralph-wiggum:cancel-ralph"
```

Or manually delete `.claude/ralph-loop.local.md`

## How It Works

1. Ralph creates `.claude/ralph-loop.local.md` with state info
2. A stop hook intercepts when Claude tries to exit
3. The same prompt is fed back to Claude
4. Claude sees its previous work in files
5. Loop continues until:
   - Promise is detected in output
   - Max iterations reached
   - Manually cancelled

## Tips

- Set realistic `--max-iterations` (15-30 for most tasks)
- Make your completion promise specific and verifiable
- Use TodoWrite to track progress within the loop
- The loop sees file changes, so commit meaningful progress to files
- If stuck, the loop will keep trying - trust the process

## Available Commands

- `/ralph-loop` or `ralph-wiggum:ralph-loop` - Start a loop
- `/cancel-ralph` or `ralph-wiggum:cancel-ralph` - Cancel active loop
- `/help` or `ralph-wiggum:help` - Show help
