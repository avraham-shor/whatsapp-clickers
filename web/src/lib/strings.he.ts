// All user-facing Hebrew copy lives here — components must never contain
// Hebrew literals (architecture rule: Hebrew copy centralization).
// Error copy follows UX-DR12: say what happened and what to do next —
// never vague, never apologizing, never blaming the user.
export const strings = {
  appTitle: 'WhatsApp Clickers',
  appTagline: 'טריוויה חיה בוואטסאפ',
  common: {
    loading: 'טוען…',
    connectionError: 'החיבור לשרת נכשל. נסו שוב בעוד רגע.',
    cancel: 'ביטול',
    // Live surfaces (lobby, and later live/display) show this only while
    // the WebSocket itself is down — never for a plain REST fetch (UX-DR12).
    connecting: 'מתחבר…',
  },
  nav: {
    myGames: 'המשחקים שלי',
    logout: 'התנתקות',
    logoutError: 'ההתנתקות מהשרת נכשלה. נסו שוב בעוד רגע.',
  },
  login: {
    title: 'כניסה ללוח הבקרה',
    usernameLabel: 'שם משתמש',
    passwordLabel: 'סיסמה',
    submit: 'כניסה',
    errorInvalidCredentials:
      'שם המשתמש והסיסמה לא תואמים לחשבון קיים. בדקו את הפרטים ונסו שוב.',
    errorServer: 'החיבור לשרת נכשל. נסו שוב בעוד רגע.',
  },
  gamesList: {
    title: 'המשחקים שלי',
    createGame: 'משחק חדש',
    joinCodeLabel: 'קוד הצטרפות',
    questionsCount: (count: number) =>
      count === 1 ? 'שאלה אחת' : `${count} שאלות`,
    emptyStateTitle: 'עוד אין כאן משחקים.',
    emptyStateBody: 'המשחק הראשון שלכם במרחק לחיצה אחת.',
    loadError: 'טעינת המשחקים נכשלה. רעננו את הדף או נסו שוב בעוד רגע.',
    finishedBadge: 'הסתיים',
  },
  createGame: {
    dialogTitle: 'משחק חדש',
    titleLabel: 'שם המשחק',
    submit: 'יצירת משחק',
    validationTitle: 'הזינו שם למשחק — עד 120 תווים.',
    errorServer: 'יצירת המשחק נכשלה. נסו שוב בעוד רגע.',
  },
  gameEditor: {
    joinCodeLabel: 'קוד הצטרפות',
    addQuestion: 'הוספת שאלה',
    openLobby: 'פתיחת הלובי',
    emptyStateTitle: 'עוד אין שאלות במשחק הזה.',
    emptyStateBody: 'הוסיפו שאלה ראשונה — או ייבאו חבילה מהמאגר.',
    emptyStateAddCta: 'הוסיפו שאלה ראשונה',
    emptyStateImportCta: 'ייבאו חבילה מהמאגר',
    typeMcq: 'רב־ברירה',
    typeFreeText: 'תשובה חופשית',
    timeLimitSeconds: (seconds: number) => `${seconds} שניות`,
    correctOption: (letter: string) => `תשובה נכונה: ${letter}`,
    acceptedAnswersCount: (count: number) =>
      count === 1 ? 'תשובה נכונה אחת' : `${count} תשובות נכונות`,
    moveUp: 'העברת השאלה מקום אחד למעלה',
    moveDown: 'העברת השאלה מקום אחד למטה',
    editQuestion: 'עריכת השאלה',
    deleteQuestion: 'מחיקת השאלה',
    deleteConfirmTitle: 'למחוק את השאלה?',
    deleteConfirmBody: 'השאלה תוסר מהמשחק. אי אפשר לבטל את המחיקה.',
    deleteConfirm: 'מחיקה',
    deleteError: 'מחיקת השאלה נכשלה. נסו שוב בעוד רגע.',
    reorderError: 'שינוי הסדר לא נשמר בשרת. הסדר הקודם שוחזר — נסו שוב.',
    notFound: 'המשחק הזה לא נמצא. ייתכן שהקישור שגוי או שהמשחק הוסר.',
    backToGames: 'חזרה למשחקים שלי',
  },
  scoring: {
    title: 'הגדרות ניקוד',
    pointsPerCorrectLabel: 'נקודות לתשובה נכונה',
    bonusLegend: 'בונוס מהירות',
    bonusHint:
      'בונוס המהירות ניתן לשלושת העונים הנכונים המהירים ביותר. ערך 0 מבטל את הבונוס.',
    bonusFirstLabel: 'מקום ראשון',
    bonusSecondLabel: 'מקום שני',
    bonusThirdLabel: 'מקום שלישי',
    save: 'שמירת הניקוד',
    saved: 'נשמר ✓',
    validation: 'הזינו מספרים שלמים בין 0 ל־10,000 בכל השדות.',
    errorServer: 'שמירת הגדרות הניקוד נכשלה. נסו שוב בעוד רגע.',
  },
  questionBank: {
    dialogTitle: 'ייבוא חבילה מהמאגר',
    openDialog: 'ייבוא מהמאגר',
    importCta: 'ייבוא למשחק',
    questionsCount: (count: number) =>
      count === 1 ? 'שאלה אחת' : `${count} שאלות`,
    importedBadge: 'מהמאגר',
    emptyTitle: 'המאגר עוד מתמלא.',
    emptyBody: 'חבילות שאלות מוכנות נמצאות בדרך — בקרוב יופיעו כאן.',
    loadError: 'טעינת המאגר נכשלה. סגרו את החלון ונסו שוב בעוד רגע.',
    importError: 'ייבוא החבילה נכשל. נסו שוב בעוד רגע.',
  },
  questionEditor: {
    createTitle: 'שאלה חדשה',
    editTitle: 'עריכת שאלה',
    typeLabel: 'סוג השאלה',
    typeMcq: 'רב־ברירה (א–ד)',
    typeFreeText: 'תשובה חופשית',
    textLabel: 'נוסח השאלה',
    optionLetters: ['א', 'ב', 'ג', 'ד'],
    optionLabel: (letter: string) => `תשובה ${letter}`,
    correctLabel: 'התשובה הנכונה',
    markCorrect: (letter: string) => `סימון תשובה ${letter} כתשובה הנכונה`,
    acceptedAnswersLabel: 'תשובות נכונות',
    acceptedAnswersHint:
      'התשובה הראשונה ברשימה היא שתוצג על המסך בחשיפת התשובה.',
    answerRowLabel: (index: number) => `תשובה נכונה ${index}`,
    addAnswer: 'הוספת תשובה',
    removeAnswer: (index: number) => `הסרת תשובה נכונה ${index}`,
    timeLimitLabel: 'מגבלת זמן לשאלה (שניות)',
    timeLimitHint:
      '30–45 שניות מתאימות לקהל רחב יותר, כולל משתתפים עם קורא מסך או קושי מוטורי.',
    submitCreate: 'הוספת השאלה',
    submitEdit: 'שמירת השינויים',
    validationText: 'הזינו את נוסח השאלה — עד 500 תווים.',
    validationOptions: 'מלאו את כל ארבע התשובות — עד 200 תווים לכל תשובה.',
    validationCorrect: 'סמנו איזו תשובה היא הנכונה.',
    validationAnswers:
      'הזינו לפחות תשובה נכונה אחת — עד 20 תשובות, עד 200 תווים לכל אחת.',
    validationTimeLimit: 'מגבלת הזמן צריכה להיות בין 5 ל־300 שניות.',
    errorServer: 'שמירת השאלה נכשלה. נסו שוב בעוד רגע.',
  },
  notFound: {
    message: 'הדף הזה לא קיים. ייתכן שהקישור שגוי או שהעמוד הוסר.',
    backHome: 'חזרה למסך הראשי',
  },
  lobby: {
    openLobbyCta: 'פתח לובי',
    openLobbyError: 'פתיחת הלובי נכשלה. נסו שוב בעוד רגע.',
    openLobbyAlreadyOpen: 'הלובי כבר פתוח. רעננו את הדף כדי לראות את המצב העדכני.',
    platformNumberLabel: 'מספר הוואטסאפ של המשחק',
    participantCountLabel: (count: number) =>
      count === 1 ? 'משתתף אחד בלובי' : `${count} משתתפים בלובי`,
  },
  live: {
    startGameCta: 'התחל משחק',
    closeQuestionCta: 'סגור שאלה',
    revealCta: 'גלה תשובה',
    nextQuestionCta: 'שאלה הבאה ←',
    stopCta: 'עצור',
    // [ASSUMPTION]: EXPERIENCE.md's State Patterns table gives the Reveal
    // row's host options as "Next / Leaderboard", but the Host-microcopy
    // table has no row for a Leaderboard CTA, so the wording is authored
    // here. 'טבלת התוצאות' matches display.stateAnnouncement.leaderboard
    // and results.leaderboardTitle character for character, so all three
    // surfaces name one thing one way.
    showLeaderboardCta: 'טבלת התוצאות',
    // EXPERIENCE.md's Host-control-panel table, "Between questions" row:
    // "פתח שאלה [N]" — the only CTA in that table carrying a number, and
    // the reason the leaderboard state cannot reuse the static
    // primaryActionByState map (see control-page.tsx).
    openQuestionNumberCta: (n: number) => `פתח שאלה ${n}`,
    // [ASSUMPTION]: EXPERIENCE.md specifies the confirm-stop dialog exists
    // but not its copy — flagged for Avraham to confirm/replace.
    stopConfirmTitle: 'לעצור את המשחק?',
    stopConfirmBody:
      'המשחק יסתיים ולא ניתן יהיה להמשיך אותו. משתתפים לא יקבלו הודעה על העצירה.',
    stopConfirmAction: 'עצור את המשחק',
    questionProgress: (n: number, total: number) => `שאלה ${n} מתוך ${total}`,
    answeredStat: (count: number) => `${count} ענו`,
    gameOverTitle: 'המשחק הסתיים',
    actionError: 'הפעולה נכשלה. נסו שוב בעוד רגע.',
    // No refresh instruction (UX-DR: no refresh dependency on live web
    // surfaces) — the WS broadcast already carries the current state; this
    // banner self-clears once it arrives.
    actionConflict: 'מצב המשחק השתנה בינתיים. הלוח יתעדכן אוטומטית.',
    gradingIncomplete: 'עדיין בודקים חלק מהתשובות. נסו שוב בעוד רגע.',
    startGameNoQuestions: 'הוסיפו לפחות שאלה אחת כדי להתחיל את המשחק.',
    // Display controls. The launch CTA matches EXPERIENCE.md's
    // Host-microcopy row character for character, but note that the row
    // itself is marked [ASSUMPTION] there and records a rejected
    // alternative ("הצג מצב מקרן") — so "verbatim" describes the
    // transcription, not a settled decision. "הפחת אנימציות" is the
    // Display-controls wording and carries no marker in the source. The
    // hint, the error and the popup-blocked message were authored to the
    // Host-microcopy rules. All four confirmed by Avraham at 4.1's code
    // review (2026-08-09), so none is [ASSUMPTION] any more.
    openDisplayCta: 'פתח מסך קהל',
    // Shown only when the browser refuses the popup — without it a
    // locked-down venue browser makes the launch CTA a silent no-op in
    // front of a room (code review, 2026-08-09).
    openDisplayBlocked: 'הדפדפן חסם את חלון מסך הקהל. אפשרו חלונות קופצים לאתר הזה ונסו שוב.',
    reduceMotionLabel: 'הפחת אנימציות',
    reduceMotionHint: 'מבטל תנועה במסך הקהל עבור כל החדר.',
    reduceMotionError: 'שינוי ההגדרה לא נשמר. נסו שוב בעוד רגע.',
  },
  // [ASSUMPTION]: EXPERIENCE.md specifies the post-game results surface
  // ("Final Leaderboard + per-question response rates on screen. No
  // export") but not its copy — authored to the Host-microcopy rules
  // (direct, terse, no apology, no emoji) and flagged for Avraham to
  // confirm/replace.
  results: {
    title: 'סיכום המשחק',
    leaderboardTitle: 'טבלת התוצאות',
    questionsTitle: 'שיעור מענה לפי שאלה',
    rankColumn: 'מקום',
    participantColumn: 'משתתף',
    scoreColumn: 'ניקוד',
    questionColumn: 'שאלה',
    answeredColumn: 'ענו',
    correctColumn: 'צדקו',
    responseRate: (answered: number, players: number) => `${answered} מתוך ${players}`,
    playerCountLabel: (count: number) =>
      count === 1 ? 'משתתף אחד שיחק' : `${count} משתתפים שיחקו`,
    noPlayers: 'אף אחד לא נרשם למשחק הזה.',
    noQuestions: 'לא היו שאלות במשחק הזה.',
    notFinished: 'המשחק עוד לא הסתיים. הסיכום יופיע כאן בסופו.',
    loadError: 'טעינת סיכום המשחק נכשלה. נסו שוב בעוד רגע.',
  },
  // EXPERIENCE.md specifies the display's resilience behavior
  // ("מתחבר מחדש..." over the last rendered state) and its stage
  // inventory, but no copy for a stage that has not been built yet or for
  // a display opened on an unknown game. Those were authored to the
  // Host-microcopy rules (direct, terse, no apology, no emoji) and
  // confirmed by Avraham at 4.1's code review (2026-08-09) — no longer
  // [ASSUMPTION].
  display: {
    // First connect, nothing rendered yet. Matches common.connecting
    // deliberately: the epic AC quotes "מתחבר...".
    connecting: 'מתחבר…',
    // Reconnect, over the last rendered stage — EXPERIENCE.md's
    // Resilience row wording.
    reconnecting: 'מתחבר מחדש…',
    notFound: 'המשחק הזה לא נמצא.',
    // Every stage until its own story lands (4.2–4.6), plus `draft`
    // permanently — a display opened before the lobby is.
    waiting: 'המסך מוכן — ממתינים למארגן.',
    // Screen-reader state names for the aria-live announcer (UX-DR14),
    // keyed by the canonical GameState strings so the switcher and the
    // announcer can never drift apart. Because `strings` is `as const`,
    // indexing this with a GameState-typed value is itself the
    // exhaustiveness check — a missing key is a compile error. Do not
    // add a `?? ''` fallback; it would disarm that check.
    stateAnnouncement: {
      draft: 'המשחק עוד לא נפתח',
      lobby: 'הרשמה פתוחה',
      question_open: 'שאלה פתוחה',
      question_closed: 'השאלה נסגרה',
      revealed: 'התשובה נחשפה',
      leaderboard: 'טבלת התוצאות',
      finished: 'המשחק הסתיים',
    },
    // Lobby stage (story 4.2). The instruction is split into two fragments
    // rather than one interpolated sentence because the two LTR runs must
    // be wrapped in <bdi> (DESIGN.md Typography: bidi isolation is
    // mandatory on web surfaces) and a plain string cannot carry markup.
    // The stage composes them as:
    //   {instructionPrefix} <bdi>JOIN {code}</bdi> {instructionTo} <bdi>{number}</bdi>
    // reproducing EXPERIENCE.md Flow 1 step 3 and mockups/key-stage-lobby.html.
    lobby: {
      instructionPrefix: 'שלחו',
      instructionTo: 'למספר',
      // [ASSUMPTION]: mockups/key-stage-lobby.html renders "57 הצטרפו".
      // Plural at every count, including 1: Hebrew singular past tense is
      // gendered ("הצטרף") and A2 forbids gendered address, so the mildly
      // imperfect "1 הצטרפו" is the gender-safe choice. Same class as
      // deferred-work.md's 1.5 "Hebrew dual-form count" entry.
      joinedLabel: 'הצטרפו',
      // Polite, throttled announcement (EXPERIENCE.md Accessibility
      // Floor). A full sentence, unlike the visual pill, because a screen
      // reader gets no layout to carry the meaning.
      countAnnouncement: (count: number) => `${count} הצטרפו למשחק`,
    },
    // Question stage (story 4.3).
    //
    // questionProgress and answeredStat are NOT redefined here: `live`
    // already holds `שאלה ${n} מתוך ${total}` and `${count} ענו`, which are
    // the exact sentences mockups/key-stage-question.html puts in the hero
    // band and EXPERIENCE.md quotes for BOTH surfaces. The question stage
    // imports them from `live`. Two blocks holding one sentence is a drift
    // risk, and this module's block names are organisational, not
    // access-controlled.
    question: {
      // EXPERIENCE.md IA > Audience Display - stages, Free-Text row, and the
      // epic AC, both verbatim. Not an assumption.
      freeTextHint: 'כתבו את התשובה בוואטסאפ',
      // The option letter as the mockup renders it ("א."). The LETTERS come
      // from questionEditor.optionLetters; only the trailing period lives
      // here, so the two surfaces cannot drift on the alphabet.
      optionLetter: (letter: string) => `${letter}.`,
      // [ASSUMPTION]: EXPERIENCE.md's Accessibility Floor requires the count
      // to be polite and throttled but gives no sentence. A full sentence,
      // unlike the visual "63 ענו", because a screen reader gets no layout to
      // carry the meaning - the same reasoning as display.lobby.countAnnouncement.
      countAnnouncement: (count: number) => `${count} ענו על השאלה`,
      // [ASSUMPTION]: EXPERIENCE.md's Accessibility Floor says the timer
      // numeral is announced "only at question open and the ≤5s threshold"
      // but gives no wording. Gender-neutral (A2), no digit read out - the
      // threshold is the event, not the number.
      urgentAnnouncement: 'חמש שניות אחרונות',
    },
    // Reveal stage (story 4.4).
    //
    // As with `question`, live.answeredStat is NOT redefined here - the
    // free-text counts line composes it with correctStat at the call site
    // rather than restating "ענו" a third time.
    reveal: {
      // DESIGN.md stage-option-correct / stage-answer-card / distribution-bar
      // all specify "✓" as the icon. One constant, so the option, the card
      // and the bar label cannot drift onto three different glyphs.
      correctMark: '✓',
      // [ASSUMPTION]: UX-DR14 requires the ✓ to be aria-labeled but gives no
      // wording. Gender-neutral (A2), states the fact rather than the glyph.
      correctOptionLabel: 'התשובה הנכונה',
      // "54 · 71%" - mockups/key-stage-reveal.html's bar label verbatim.
      // Rendered inside <bdi dir="ltr"> by the stage: it is an all-digit run
      // in an RTL paragraph and the bidi algorithm has no strong character
      // to anchor it (DESIGN.md Typography: bidi isolation is mandatory).
      distributionLabel: (count: number, percent: number) => `${count} · ${percent}%`,
      // The "Y צדקו" half of EXPERIENCE.md's Free-Text reveal counts line
      // "X ענו · Y צדקו"; the "X ענו" half is strings.live.answeredStat.
      // Near-duplicate of results.correctColumn ('צדקו'), deliberately not
      // reused: that one is a table COLUMN HEADER on the dashboard and a
      // later copy change to either surface must not silently move the other.
      correctStat: (count: number) => `${count} צדקו`,
    },
    // Leaderboard stage (story 4.5).
    leaderboard: {
      // DESIGN.md leaderboard-row-mover: "▲ + places climbed". One
      // constant so the glyph cannot drift, the same discipline as
      // reveal.correctMark.
      moverMark: '▲',
      // [ASSUMPTION]: UX-DR14 requires the indicator to be aria-labeled
      // but gives no wording. Noun form so there is no gendered verb
      // (A2: 'עלה' is masculine); the singular is spelled out for the
      // same reason results.playerCountLabel spells out 'משתתף אחד'.
      climbedLabel: (places: number) =>
        places === 1 ? 'עלייה של מקום אחד' : `עלייה של ${places} מקומות`,
      // [ASSUMPTION]: reachable — a game can be started and played with
      // no registered players (nothing gates start on a roster), and
      // GetLeaderboard then returns zero rows. Distinct from the degraded
      // frame, which renders display.waiting instead (story 4.5, derived
      // requirement 8). Impersonal, matching results.noPlayers rather
      // than restating it: that one is a dashboard empty state and a
      // later copy change to either must not move the other.
      noPlayers: 'אף אחד לא נרשם למשחק הזה.',
    },
    // Winner takeover (story 4.6). Mirrors messages_he.go's Winner's-final-
    // message tone ("מזל טוב, [שם]! 🏆 ניצחת עם [ניקוד] נקודות!") without
    // repeating its exact wording — the projector composes name and score as
    // separate visual elements (mockups/key-stage-winner.html), not one
    // interpolated sentence.
    winner: {
      // mockups/key-stage-winner.html's w-tagline, verbatim. Gender/number
      // neutral, so it needs no tie/no-tie variant.
      tagline: 'מזל טוב!',
      // [ASSUMPTION]: no score-suffix string exists yet for the display
      // surface (messages_he.go's templates interpolate "X נקודות" inline
      // in Go, not as a reusable fragment). One function so the digit-plus-
      // suffix cannot drift from results.playerCountLabel's spelled-out-
      // singular discipline elsewhere in this file — though a score has no
      // natural "one point" singular concern the way a participant count
      // does, so this is a plain suffix, not a pluralizing function.
      //
      // Rendered by winner-stage.tsx inside a PLAIN <bdi> (dir="auto"), not
      // <bdi dir="ltr">: the composed value is a Hebrew sentence whose first
      // strong character is Hebrew, so forcing LTR would mirror it and put
      // the digits on the wrong side of the word. Auto isolates the run from
      // its surroundings while resolving the direction correctly, and the
      // digit run inside keeps its own LTR order by the bidi algorithm.
      scoreSuffix: (score: number) => `${score} נקודות`,
      // Distinct from display.leaderboard.noPlayers on purpose: that one
      // describes an EMPTY roster; this one describes a roster where
      // nobody's score cleared game/final.go's strict positivity condition
      // (a stopped-before-first-reveal game, or genuinely nobody registered).
      // Both collapse to "no winner to announce" and get one honest
      // sentence, mirroring messages_he.go's msgFinalResultsNoWinner rather
      // than inventing new wording for a case WhatsApp already names.
      noWinner: 'המשחק נגמר! הפעם לא נצברו נקודות.',
    },
  },
} as const
