# canvas-report

A command-line tool for parents to see what's missing, what's due today, what's coming up this week, and current grades in Canvas. Supports multiple children.

## Example

```
$ canvas-report
[✔] Jane Doe: 12/12 courses... done (256 assignments)
[✔] Tommy Doe: 9/9 courses... done (226 assignments)

┌────────────────────────────────────────┐
│ Jane Doe                               │
│ Generated: Sat Jan 10, 2026 at 7:13 PM │
└────────────────────────────────────────┘

MISSING/INCOMPLETE (3)
┌────────────────────────────────┬──────────────────────────────────────────────────────┬───────────────────┬───────┬─────┐
│ Subject                        │ Assignment                                           │ Due               │   Pts │     │
├────────────────────────────────┼──────────────────────────────────────────────────────┼───────────────────┼───────┼─────┤
│ Health & PE                    │ Safety & Violence Prevention Warm ups                │ thu 12/18 11pm    │     8 │ 0   │
│ English Language Arts          │ Vision Board Organizer                               │ tue 1/6 2pm       │    20 │ ✗   │
│ Pre-Algebra                    │ 8.4.4 practice                                       │ fri 1/9 8am       │     5 │ 0   │
└────────────────────────────────┴──────────────────────────────────────────────────────┴───────────────────┴───────┴─────┘

DUE TODAY/TOMORROW (1 pending)
┌────────────────────────────────┬──────────────────────────────────────────────────────┬───────────────────┬───────┬─────┐
│ English Language Arts          │ Vision Board                                         │ mon 1/12 8am      │    65 │ ✓   │
│ Health & PE                    │ Puberty Quiz                                         │ mon 1/12 2pm      │    10 │     │
└────────────────────────────────┴──────────────────────────────────────────────────────┴───────────────────┴───────┴─────┘

WEEK AHEAD (2 pending)
┌────────────────────────────────┬──────────────────────────────────────────────────────┬───────────────────┬───────┬─────┐
│ Pre-Algebra                    │ 8.4.5 Practice Problems                              │ tue 1/13 8am      │     6 │     │
│ Geography                      │ Life in West Africa Comic Strip                      │ wed 1/14 11pm     │    12 │     │
└────────────────────────────────┴──────────────────────────────────────────────────────┴───────────────────┴───────┴─────┘

GRADES - Q2 (Oct 14 - Jan 17)
┌─────────────────────────────┬────────┬──────────┬─────────┐
│ Subject                     │ Points │ Possible │       % │
├─────────────────────────────┼────────┼──────────┼─────────┤
│ English Language Arts       │    512 │      550 │  93.09% │
│ Geography                   │    445 │      480 │  92.71% │
│ Health & PE                 │    180 │      200 │  90.00% │
│ Pre-Algebra                 │    423 │      485 │  87.22% │
└─────────────────────────────┴────────┴──────────┴─────────┘

3 missing | 1 due soon | 2 this week

═══════════════════════════════════════════════════════════════════════════════

┌────────────────────────────────────────┐
│ Tommy Doe                              │
│ Generated: Sat Jan 10, 2026 at 7:13 PM │
└────────────────────────────────────────┘

MISSING/INCOMPLETE (0)
  All caught up!

DUE TODAY/TOMORROW (2 pending)
┌────────────────────────────────┬──────────────────────────────────────────────────────┬───────────────────┬───────┬─────┐
│ Biology                        │ Cell Division Worksheet                              │ mon 1/12 8am      │    15 │     │
│ US History                     │ Chapter 12 Reading Questions                         │ mon 1/12 3pm      │    10 │     │
└────────────────────────────────┴──────────────────────────────────────────────────────┴───────────────────┴───────┴─────┘

WEEK AHEAD (1 pending)
┌────────────────────────────────┬──────────────────────────────────────────────────────┬───────────────────┬───────┬─────┐
│ Algebra II                     │ Quadratic Functions Quiz                             │ thu 1/15 10am     │    25 │     │
└────────────────────────────────┴──────────────────────────────────────────────────────┴───────────────────┴───────┴─────┘

GRADES - Q2 (Oct 14 - Jan 17)
┌─────────────────────────────┬────────┬──────────┬─────────┐
│ Subject                     │ Points │ Possible │       % │
├─────────────────────────────┼────────┼──────────┼─────────┤
│ Algebra II                  │    567 │      600 │  94.50% │
│ Biology                     │    482 │      500 │  96.40% │
│ US History                  │    438 │      475 │  92.21% │
└─────────────────────────────┴────────┴──────────┴─────────┘

0 missing | 2 due soon | 1 this week
```

## Setup

### 1. Prerequisites

- Parent/Observer access to your child's Canvas account (contact your school)
- A Canvas API token

### 2. Get a Canvas API Token

1. Log into Canvas with your **parent** account
2. Click your profile picture → **Settings**
3. Scroll to **Approved Integrations** → **+ New Access Token**
4. Enter a purpose (e.g., "canvas-report")
5. Click **Generate Token** and copy it immediately

### 3. Install

Download the binary for your platform from [Releases](https://github.com/rcresswell/canvas-report/releases), or build from source:

```bash
git clone https://github.com/rcresswell/canvas-report.git
cd canvas-report
go build
```

### 4. Run

```bash
./canvas-report        # macOS/Linux
canvas-report.exe      # Windows
```

First run will prompt for your Canvas URL and API token, then save them to `~/.config/canvas-report/config.yaml` (or `%USERPROFILE%\.config\canvas-report\config.yaml` on Windows).

## Options

- `--all` - Include assignments older than 30 days in the missing section

## License

MIT
