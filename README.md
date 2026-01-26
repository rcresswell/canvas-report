# canvas-report

A command-line tool for parents to see what's missing, what's due today, what's coming up this week, and current grades in Canvas. Supports multiple children.

## Example

```
$ ./canvas-report
[✔] Jane Doe: 12/12 courses... done (256 assignments)
[✔] Tommy Doe: 9/9 courses... done (226 assignments)

┌────────────────────────────────────────┐
│ Jane Doe                               │
│ Generated: Sat Jan 10, 2026 at 7:13 PM │
└────────────────────────────────────────┘

MISSING/INCOMPLETE (3)
┌──────────────────────┬───────────────────────────────────────────────┬───────────────────┬─────┬─────────────┬───┐
│ Subject              │ Assignment                                    │ Due               │ Pts │ Impact      │   │
├──────────────────────┼───────────────────────────────────────────────┼───────────────────┼─────┼─────────────┼───┤
│ Health & PE          │ Safety & Violence Prevention Warm ups         │ thu 12/18 11pm    │   8 │ +4.0%       │ 0 │
│ English Language A…  │ Vision Board Organizer (Formative)            │ tue 1/6 2pm       │  20 │ +0.8/-1.2%  │ ✗ │
│ Pre-Algebra          │ 8.4.4 practice (Homework)                     │ fri 1/9 8am       │   5 │ -           │ 0 │
└──────────────────────┴───────────────────────────────────────────────┴───────────────────┴─────┴─────────────┴───┘

DUE TODAY/TOMORROW (1 pending)
┌──────────────────────┬───────────────────────────────────────────────┬───────────────────┬─────┬─────────────┬───┐
│ Subject              │ Assignment                                    │ Due               │ Pts │ Impact      │   │
├──────────────────────┼───────────────────────────────────────────────┼───────────────────┼─────┼─────────────┼───┤
│ English Language A…  │ Vision Board (Summative)                      │ mon 1/12 8am      │  65 │             │ ✓ │
│ Health & PE          │ Fitness Log Week 12                           │ mon 1/12 2pm      │  10 │ +2.1/-1.8%  │   │
└──────────────────────┴───────────────────────────────────────────────┴───────────────────┴─────┴─────────────┴───┘

WEEK AHEAD (2 pending)
┌──────────────────────┬───────────────────────────────────────────────┬───────────────────┬─────┬─────────────┬───┐
│ Subject              │ Assignment                                    │ Due               │ Pts │ Impact      │   │
├──────────────────────┼───────────────────────────────────────────────┼───────────────────┼─────┼─────────────┼───┤
│ Pre-Algebra          │ 8.4.5 Practice Problems (Classwork)           │ tue 1/13 8am      │   6 │ +0.3/-0.2%  │   │
│ Geography            │ Life in West Africa Comic Strip (Projects)    │ wed 1/14 11pm     │  12 │ +1.5/-1.1%  │   │
└──────────────────────┴───────────────────────────────────────────────┴───────────────────┴─────┴─────────────┴───┘

GRADES - Q2 (Oct 14 - Jan 17)
┌─────────────────────────────┬─────────┬────────┬──────────┬────────┐
│ Subject                     │       % │ Points │ Possible │ Weight │
├─────────────────────────────┼─────────┼────────┼──────────┼────────┤
│ English Language Arts       │  93.09% │        │          │        │
│   Formative                 │  95.00% │    190 │      200 │    40% │
│   Homework                  │ 100.00% │     50 │       50 │    10% │
│   Summative                 │  90.67% │    272 │      300 │    50% │
│ Geography                   │  92.71% │    445 │      480 │        │
│ Health & PE                 │  90.00% │    180 │      200 │        │
│ Pre-Algebra                 │  87.22% │    423 │      485 │        │
└─────────────────────────────┴─────────┴────────┴──────────┴────────┘

3 missing | 1 due soon | 2 this week

═══════════════════════════════════════════════════════════════════════════════

┌────────────────────────────────────────┐
│ Tommy Doe                              │
│ Generated: Sat Jan 10, 2026 at 7:13 PM │
└────────────────────────────────────────┘

MISSING/INCOMPLETE (0)
  All caught up!

DUE TODAY/TOMORROW (2 pending)
┌──────────────────────┬───────────────────────────────────────────────┬───────────────────┬─────┬─────────────┬───┐
│ Subject              │ Assignment                                    │ Due               │ Pts │ Impact      │   │
├──────────────────────┼───────────────────────────────────────────────┼───────────────────┼─────┼─────────────┼───┤
│ Biology              │ Cell Division Worksheet (Labs)                │ mon 1/12 8am      │  15 │ +0.6/-0.5%  │   │
│ US History           │ Chapter 12 Reading Questions (Homework)       │ mon 1/12 3pm      │  10 │ -           │   │
└──────────────────────┴───────────────────────────────────────────────┴───────────────────┴─────┴─────────────┴───┘

WEEK AHEAD (1 pending)
┌──────────────────────┬───────────────────────────────────────────────┬───────────────────┬─────┬─────────────┬───┐
│ Subject              │ Assignment                                    │ Due               │ Pts │ Impact      │   │
├──────────────────────┼───────────────────────────────────────────────┼───────────────────┼─────┼─────────────┼───┤
│ Algebra II           │ Quadratic Functions Quiz (Tests)              │ thu 1/15 10am     │  25 │ +1.2/-0.9%  │   │
└──────────────────────┴───────────────────────────────────────────────┴───────────────────┴─────┴─────────────┴───┘

GRADES - Q2 (Oct 14 - Jan 17)
┌─────────────────────────────┬─────────┬────────┬──────────┬────────┐
│ Subject                     │       % │ Points │ Possible │ Weight │
├─────────────────────────────┼─────────┼────────┼──────────┼────────┤
│ Algebra II                  │  94.50% │    567 │      600 │        │
│ Biology                     │  96.40% │    482 │      500 │        │
│ US History                  │  92.21% │    438 │      475 │        │
└─────────────────────────────┴─────────┴────────┴──────────┴────────┘

0 missing | 2 due soon | 1 this week
```

## Features

### Grade Impact

The **Impact** column shows how much each assignment could affect the overall class grade:

- `+1.2/-0.9%` — Getting 100% would raise the grade by 1.2%; getting 0% would lower it by 0.9%
- `+4.0%` — Only upside (assignment already graded as 0, so completing it can only help)
- `-1.5%` — Only downside (e.g., a missing assignment in a high-scoring category)
- `-` — No impact (zero-weight category like ungraded homework)

This helps prioritize which assignments matter most for the grade.

### Weighted Categories

For courses that use weighted grading, the assignment name shows its category in parentheses (e.g., "Essay Draft (Formative)"). The grades section breaks down each weighted category with its percentage and weight.

### Status Icons

- `✓` — Completed (submitted or graded)
- `✗` — Missing (not submitted, past due)
- `0` — Graded as zero

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
