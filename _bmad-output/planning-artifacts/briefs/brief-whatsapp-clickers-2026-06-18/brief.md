---
title: Product Brief — whatsapp-clickers
status: draft
created: 2026-06-18
updated: 2026-06-18
---

# Product Brief: WhatsApp Clickers

## Executive Summary

WhatsApp Clickers is a real-time interactive quiz platform built for the Israeli market. Organizers — school principals, HR managers, event coordinators — create and run quiz sessions for groups of up to 500 participants. Participants register by sending a single WhatsApp message: no app download, no account creation, no URL to navigate to. They already have WhatsApp open.

Existing tools fail this market in two ways: IVR-based systems (call a number, press keys) are uncomfortable, slow, and limited to numeric answers; global platforms like Kahoot are not built for Hebrew and RTL text. WhatsApp Clickers is Hebrew-native, WhatsApp-first, and supports free-text questions with AI-assisted answer validation — making it the only quiz platform built specifically for Hebrew-speaking audiences.

Organizers pay a flat fee per program, purchase directly on the website, and get immediate access to the platform.

## The Problem

Running a group quiz for a large audience in Israel is harder than it should be. Two categories of tools exist — and neither works well.

**IVR phone systems** require participants to call a number and press keypad digits to answer. Holding a phone to your ear for twenty minutes during a group activity is uncomfortable. You can only ask multiple-choice questions with numeric answers. There is no visual feedback, no score display, no leaderboard energy in the room.

**Global web platforms** like Kahoot require participants to navigate to a URL and enter a game code in a browser. For 500 people at a corporate team day or school event, coordinating that login simultaneously is friction. More critically, these platforms are built for English and left-to-right languages; Hebrew content is possible but cumbersome. `[ASSUMPTION: Hebrew experience in Kahoot is poor enough that it is a real barrier — to be validated with target users.]`

The result: organizers either tolerate a clunky IVR experience or struggle to use a platform not designed for their language.

## The Solution

WhatsApp Clickers replaces the registration problem with a single message.

Participants send `JOIN <code>` to a designated WhatsApp number. The system identifies them by phone number, registers them for the session, and confirms in return. That is it. No browser, no account, no app. For Israeli participants who already have WhatsApp on their phone and use it constantly, this is the lowest-friction action possible.

From there, participants join a dedicated web interface to view questions, see a countdown timer, submit answers, and follow the live leaderboard.

**Question types:**
- **Multiple choice** — single correct answer selected from options.
- **Free text** — participants type an answer in their own words (e.g., "Who was the first Prime Minister of Israel?").

**Free-text answers are validated in three stages:**
1. Exact match against host-defined accepted answers.
2. Fuzzy match for spelling variations and typos.
3. AI semantic validation for equivalent wording — used only when the first two stages fail.

The host defines all accepted answers. The AI never generates correct answers — it only determines whether a participant's response sufficiently matches what the host specified.

**Scoring** rewards both correctness and speed: a configurable point value for correct answers, with bonus points for the first, second, and third correct responses.

## What Makes This Different

**Hebrew-native, not Hebrew-compatible.** The platform is built from the ground up for RTL text, Hebrew characters, and Hebrew language AI validation. Global tools treat Hebrew as an afterthought.

**WhatsApp as identity layer.** In Israel, WhatsApp penetration is among the highest in the world. Using WhatsApp for registration means every participant already has the client installed, knows how to use it, and their phone number becomes their identity — no logins, no forgotten passwords, no registration forms.

**Free text with AI validation.** No existing Israeli quiz platform supports open-ended questions at scale with automated validation. This unlocks a richer class of quiz — trivia, knowledge competitions, educational assessments — where multiple-choice alone isn't enough.

**Built for live events.** Real-time leaderboards, countdown timers, and instant answer feedback are designed for the energy of a room, not an async learning module.

## Who This Serves

**Primary buyer — the institutional organizer:**
A school principal running an end-of-year program, an HR manager organizing a company team day, or an event coordinator planning a group activity. They need a quiz experience that works for their audience without requiring technical setup from participants. They find and purchase the platform online, pay a flat fee per program, and get immediate access.

**Participant — the audience member:**
Students, employees, or event attendees. Typically non-technical; they should not need to learn anything new. Their only required action before the event is sending one WhatsApp message. During the event they interact through a mobile browser.

**Scale:** Up to 500 concurrent participants per session.

## Success Criteria

`[ASSUMPTION: The following success signals were not confirmed by the founder — to be validated.]`

**For the organizer (user success):**
- Participants connect in under two minutes at the start of the event.
- Zero participants fail to register due to technical friction.
- The host can create and run a full quiz without external support.
- After the event, the organizer feels it went smoothly and would use the platform again.

**For the business:**
- `[ASSUMPTION]` Paying customers within the first quarter of launch.
- Net Promoter Score from organizers sufficient to drive word-of-mouth in the Israeli education and corporate events market.
- Repeat purchase rate — organizers returning for a second program.

## Scope

**In for v1:**
- WhatsApp-based participant registration (JOIN flow).
- Host dashboard: create games, create and manage questions (MCQ and free text), open/close questions, view live results.
- Participant web interface: view question, countdown timer, submit answer, see personal score and leaderboard.
- Three-stage answer validation (exact → fuzzy → AI semantic) for free-text questions.
- Configurable scoring rules (points per correct answer, speed bonuses).
- Real-time updates throughout (participant count, answers received, live rankings).
- Result export.
- Hebrew-first UI throughout.
- Self-serve purchase and access on the website.

**Explicitly out of v1:**
- Mobile app (web-only for participants).
- WhatsApp-delivered question display (participants view questions on the web interface, not in WhatsApp).
- Multi-language support beyond Hebrew.
- Advanced analytics and reporting beyond basic export.
- Team/group scoring modes.
- `[ASSUMPTION: anything else from the original requirements list not named above.]`

## Vision

In two to three years, WhatsApp Clickers becomes the standard quiz and audience-response platform for Hebrew-speaking institutions — the tool Israeli schools and corporate trainers reach for automatically, the way English-speaking markets default to Kahoot.

From that base, the platform can expand: richer question types, integrations with school management systems, white-label options for event companies, and potentially Arabic-language support to reach broader MENA markets where WhatsApp penetration is similarly high.

The long-term bet is that WhatsApp as an identity and registration layer, combined with strong local-language AI, is a model that can repeat across underserved language markets globally.
