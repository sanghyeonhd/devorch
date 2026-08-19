#!/usr/bin/env python3
"""Simulate the chat-first plan against a Codex/Claude quality bar.

This does not implement a product. It inspects the current tree, walks
synthetic user journeys, and scores architecture options. Findings feed
DEVORCH_3_CHAT_GUI_AND_REUSE.md.

Run from the repository root:

    python3 docs/upgrade/devorch_chat_surface_simulation.py
"""

from __future__ import annotations

import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]


def read(rel: str) -> str:
    path = ROOT / rel
    return path.read_text(encoding="utf-8") if path.exists() else ""


def count_matches(pattern: str, text: str) -> int:
    return len(re.findall(pattern, text, flags=re.MULTILINE))


# --- inventory: measured, not guessed ---------------------------------------

cli = read("bins/devorch/src/main.rs")
ui_state = read("crates/devorch-ui/src/state.rs")
ui_views = read("crates/devorch-ui/src/views.rs")
ui_app = read("crates/devorch-ui/src/app.rs")
cargo = read("Cargo.toml")
event = read("crates/devorch-protocol/src/event.rs")
runner = read("crates/devorch-agent/src/runner.rs")
plan = read("docs/upgrade/DEVORCH_3_CHAT_GUI_AND_REUSE.md")

inventory = {
    "cli_is_clap_subcommands": "struct Cli" in cli and "Subcommand" in cli,
    "cli_default_is_tui": "ratatui" in cli.lower() or "tui" in cli.lower(),
    "cli_has_chat_command": bool(re.search(r"\bChat\b", cli)),
    "gui_default_screen_is_mission": "MissionControl" in ui_state
    and "Default" in ui_state,
    "gui_has_chat_screen": "Screen::Chat" in ui_state or "Chat" in ui_state
    and "Screen::Chat" in ui_views,
    "gui_composer_is_single_line_goal": "TextEdit::singleline" in ui_views
    and "form.goal" in ui_views,
    "gui_has_multiline_composer": "TextEdit::multiline" in ui_views,
    "depends_on_ratatui": "ratatui" in cargo,
    "depends_on_egui": "egui" in cargo,
    "depends_on_egui_commonmark": "egui_commonmark" in cargo,
    "agent_event_has_text_delta": "TextDelta" in event,
    "agent_event_has_permission": "PermissionRequired" in event,
    "agent_event_has_tool_cards": "ToolStarted" in event and "ToolCompleted" in event,
    "run_agent_is_one_shot": "pub struct AgentRun" in runner
    and "events: Vec<AgentEvent>" in runner,
    "session_journal_exists": (ROOT / "crates/devorch-store/src/journal.rs").exists(),
    "chat_session_kind_in_migrations": "chat_session"
    in read("crates/devorch-store/migrations/0002_documents.sql"),
    "plan_claims_four_days": "목표: 하루" in plan and "S1 — 채팅 홈" in plan,
    "plan_puts_chat_in_egui_first": "Screen::Chat을 기본으로" in plan,
    "plan_cli_is_repl_not_tui": "echo \"인증 테스트 고쳐\" | devorch chat" in plan,
    "revised_chooses_tui_first": "B. 코어 + ratatui TUI가 제품" in plan
    or "B_tui_first_rust" in plan
    or "TUI가 홈" in plan,
    "revised_forbids_ui_chat_rs": "crates/devorch-ui/src/chat.rs" in plan
    and "만들지 않음" in plan,
    "revised_t0_before_t1": "### T0 — 스레드가 되게" in plan
    and plan.find("### T0") < plan.find("### T1"),
    "revised_resume_on_execute_request": "ExecuteRequest" in plan
    and "resume" in plan,
}

# Codex/Claude daily-driver features. Desktop of those products is either
# proprietary (ChatGPT app, Claude.app, VS Code) or a client of the same
# session protocol as the TUI. The open, matchable bar is the TUI.

quality_bar = [
    {
        "id": "binary_opens_chat",
        "surface": "terminal",
        "codex": "codex  (no args) opens ratatui TUI",
        "claude": "claude (no args) opens Ink TUI",
        "plan": "devorch-ui window, mission form; CLI is clap",
        "covered_now": False,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": True,
    },
    {
        "id": "streaming_transcript",
        "surface": "both",
        "codex": "token stream into last assistant card",
        "claude": "same",
        "plan": "coalesce TextDelta into bubbles",
        "covered_now": False,
        "covered_by_s1_egui": True,
        "covered_by_tui_first": True,
    },
    {
        "id": "multiline_composer",
        "surface": "both",
        "codex": "Enter send / paste / wrapped whitespace / CRLF (0.148)",
        "claude": "multiline + paste images",
        "plan": "egui multiline in desktop only; CLI is a REPL line",
        "covered_now": False,
        "covered_by_s1_egui": True,
        "covered_by_tui_first": True,
    },
    {
        "id": "slash_commands",
        "surface": "terminal",
        "codex": "/export /compact /model palette",
        "claude": "/commit /review /mcp ...",
        "plan": "/compare /session from Go TUI names",
        "covered_now": False,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": True,
    },
    {
        "id": "session_resume",
        "surface": "both",
        "codex": "resume restores cwd, policy, transcript preview",
        "claude": "--resume / session picker",
        "plan": "documents kind=chat_session (not built)",
        "covered_now": False,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": True,
    },
    {
        "id": "tool_and_approval_cards",
        "surface": "both",
        "codex": "tool + approval queue in TUI",
        "claude": "permission prompts inline",
        "plan": "mention policy cards; GUI has no approval queue",
        "covered_now": "PermissionRequired" in event,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": True,
    },
    {
        "id": "interrupt",
        "surface": "both",
        "codex": "Esc cancels the turn",
        "claude": "Ctrl+C / Esc",
        "plan": "not specified",
        "covered_now": False,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": True,
    },
    {
        "id": "markdown_code_fences",
        "surface": "both",
        "codex": "transcript markdown, /export md",
        "claude": "rich markdown in TUI",
        "plan": "egui_commonmark (desktop); nothing for terminal",
        "covered_now": False,
        "covered_by_s1_egui": True,
        "covered_by_tui_first": True,
    },
    {
        "id": "multi_turn_same_vendor_session",
        "surface": "both",
        "codex": "persistent thread, --resume",
        "claude": "same process or resume id",
        "plan": "run_agent one-shot per send (AgentRun)",
        "covered_now": False,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": False,
    },
    {
        "id": "desktop_equals_tui_protocol",
        "surface": "desktop",
        "codex": "app-server JSON-RPC, same core as TUI",
        "claude": "VS Code / .app talk to same CLI session",
        "plan": "egui is a second chat renderer of run_agent",
        "covered_now": False,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": True,
    },
    {
        "id": "native_desktop_visual_polish",
        "surface": "desktop",
        "codex": "ChatGPT desktop is proprietary, not in openai/codex",
        "claude": "Claude.app / VS Code proprietary",
        "plan": "egui_commonmark in 1 day",
        "covered_now": False,
        "covered_by_s1_egui": False,
        "covered_by_tui_first": False,
    },
]


def score(option: str) -> dict:
    """Burden = extra toolchains + dual UIs. Speed = days to a usable chat.
    Ceiling = how close to Codex TUI / Claude TUI, 0-10.
    """
    table = {
        "A_egui_chat_four_days": {
            "label": "Current plan: egui chat home in 4 days, clap REPL",
            "stay_rust": True,
            "days_to_usable_chat": 4,
            "days_to_codex_tui_parity": None,
            "burden": "low toolchain, high dual-UI debt later",
            "tui_ceiling": 2,
            "desktop_ceiling": 4,
            "matches_user_bar": False,
            "why": "egui can show bubbles; it cannot become Codex TUI. CLI stays a form/REPL. A later ratatui would duplicate chat.rs.",
        },
        "B_tui_first_rust": {
            "label": "Rust core + ratatui TUI as the product; egui stays Compare/Agents",
            "stay_rust": True,
            "days_to_usable_chat": 10,
            "days_to_codex_tui_parity": 30,
            "burden": "one language, one extra crate family (ratatui/crossterm)",
            "tui_ceiling": 8,
            "desktop_ceiling": 5,
            "matches_user_bar": True,
            "why": "Codex is the existence proof that a Rust TUI can be the daily driver. Desktop polish stays below Claude.app; interaction quality can match.",
        },
        "C_opencode_desktop_shell": {
            "label": "Rust core + OpenCode/Tauri or SolidJS as the chat shell",
            "stay_rust": False,
            "days_to_usable_chat": 14,
            "days_to_codex_tui_parity": 21,
            "burden": "Bun/Node + Rust, two CI worlds, license PORT of a moving target",
            "tui_ceiling": 7,
            "desktop_ceiling": 8,
            "matches_user_bar": True,
            "why": "Fastest visual desktop. Raises burden, which is the opposite of 'Rust to reduce load'. TUI would still be missing unless also adopted.",
        },
        "D_two_chats": {
            "label": "egui chat AND ratatui TUI as separate implementations",
            "stay_rust": True,
            "days_to_usable_chat": 20,
            "days_to_codex_tui_parity": 40,
            "burden": "rule 5 violation: two composers, two session lists",
            "tui_ceiling": 8,
            "desktop_ceiling": 5,
            "matches_user_bar": False,
            "why": "Looks complete, drifts immediately. Codex does not do this: TUI and desktop share app-server.",
        },
    }
    return {"id": option, **table[option]}


# --- journeys: walk the current plan, record where it breaks ----------------

journeys = []


def journey(name, steps, defect=None):
    journeys.append({"name": name, "steps": steps, "defect": defect})


journey(
    "first_open_like_codex",
    [
        "user types `devorch`",
        "expected: full-screen chat TUI",
        "actual: clap help (no default command is a TUI)",
        "S1 actual: opens egui Mission Control unless Screen default is changed",
        "S1 still does not make `devorch` a TUI",
    ],
    defect="PLAN-1: the daily driver in Codex/Claude is the terminal binary, not a separate GUI. The plan inverts that.",
)

journey(
    "type_and_stream",
    [
        "user pastes a 40-line stack trace",
        "Codex composer accepts CRLF paste and wrap",
        "current GUI: single-line Goal field",
        "S1 egui: multiline, yes",
        "S1 CLI: `devorch chat` described as REPL, not a composer",
    ],
    defect="PLAN-2: `devorch chat` as a line REPL is not Codex-level. The terminal surface must be a TUI composer.",
)

journey(
    "second_turn_same_thread",
    [
        "user: 'fix the test'",
        "agent edits a file, process exits (OneShotStructured)",
        "user: 'also add a docstring'",
        "Codex: same thread, vendor session id",
        "plan S1: second run_agent() with no vendor --resume wiring",
    ],
    defect="PLAN-3: multi-turn is not a view change. Adapter ExecuteRequest must carry vendor session id; one-shot-per-send loses the thread.",
)

journey(
    "permission_inline",
    [
        "agent wants to run a command outside the worktree",
        "policy emits Ask / PermissionRequired",
        "Codex: inline approval in the transcript",
        "current GUI: no Command for allow/deny on a live turn",
        "plan: 'policy cards' with no event loop",
    ],
    defect="PLAN-4: approval is a turn-blocking protocol, not a Settings screen. Missing from S1–S4.",
)

journey(
    "interrupt",
    [
        "agent is streaming",
        "user hits Esc",
        "plan never mentions killing the child",
        "devorch-process has Timeout but no user-initiated cancel in UI",
    ],
    defect="PLAN-5: without interrupt, a chat is not usable at Codex/Claude level.",
)

journey(
    "resume_after_quit",
    [
        "quit, reopen, pick yesterday's session",
        "event journal can replay by SessionId",
        "plan stores a new chat_session document",
        "two stores of the same conversation unless they are the same rows",
    ],
    defect="PLAN-6: do not add chat_session if the journal already is the transcript. Replay events; store only session metadata (title, repo, agents).",
)

journey(
    "compare_from_chat",
    [
        "/compare high fix auth",
        "MissionRunner exists and is tested",
        "this path is real",
    ],
    defect=None,
)

journey(
    "desktop_claude_app_look",
    [
        "user expects Claude.app / ChatGPT desktop chrome",
        "those UIs are not open source",
        "egui_commonmark in one day produces a dashboard chat, not that product",
    ],
    defect="PLAN-7: matching proprietary desktop chrome is out of scope. Matching their TUI interaction is in scope and is Rust-shaped.",
)


defects = [j["defect"] for j in journeys if j["defect"]]

options = [
    score("A_egui_chat_four_days"),
    score("B_tui_first_rust"),
    score("C_opencode_desktop_shell"),
    score("D_two_chats"),
]

# Verdict: stay Rust, but not the 4-day egui plan.
revised_ok = (
    inventory["revised_chooses_tui_first"]
    and inventory["revised_forbids_ui_chat_rs"]
    and inventory["revised_t0_before_t1"]
    and inventory["revised_resume_on_execute_request"]
    and not inventory["plan_puts_chat_in_egui_first"]
)

verdict = {
    "stay_rust": True,
    "stay_egui_as_chat_home": False,
    "chosen": "B_tui_first_rust",
    "document_revised": revised_ok,
    "because": [
        "Codex, the quality bar named by the user, is a Rust ratatui TUI. Staying Rust is how you copy that bar rather than fight it.",
        "Rust already carries process, adapters, policy, verify, mission. Adding Bun/Tauri increases burden, which the user asked to reduce.",
        "egui is the wrong primary chat surface: no TUI, weak selection/copy, and a second implementation if a TUI is added later.",
        "Claude.app visual polish is not available to port. Claiming it in four days is a plan failure, same class as 'merge after tests' ignoring untracked files.",
        "/compare on MissionRunner is the one part of the current plan that is correctly scoped.",
    ],
    "what_egui_is_for": "Compare, Constellation, Agent board, Policy inspector — a companion, not the composer.",
    "integrator_dependency_requests": [
        "ratatui",
        "crossterm",
        "tui-textarea or equivalent composer (MIT/Apache; record in upstream-sources.toml if PORT)",
    ],
}

report = {
    "result": (
        "PLAN REVISED — TUI-first, do not implement yet"
        if revised_ok
        else "REVISE THE PLAN — do not execute S1–S4 as written"
    ),
    "inventory": inventory,
    "quality_bar": quality_bar,
    "journeys": journeys,
    "defects": defects,
    "options": options,
    "verdict": verdict,
    "bar_coverage": {
        "now": sum(1 for f in quality_bar if f["covered_now"] is True),
        "plan_s1_egui": sum(1 for f in quality_bar if f["covered_by_s1_egui"] is True),
        "tui_first": sum(1 for f in quality_bar if f["covered_by_tui_first"] is True),
        "total": len(quality_bar),
    },
}

out = ROOT / "docs/upgrade/devorch_chat_surface_simulation.json"
out.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

print(f"result: {report['result']}")
print(f"bar coverage now/s1-egui/tui-first: {report['bar_coverage']}")
print(f"defects ({len(defects)}):")
for d in defects:
    print(f"  - {d}")
print(f"chosen: {verdict['chosen']}")
print(f"wrote {out.relative_to(ROOT)}")

if not inventory["cli_is_clap_subcommands"]:
    sys.exit("inventory failed: CLI is not clap")
if inventory["depends_on_ratatui"]:
    sys.exit("inventory surprise: ratatui already present")
