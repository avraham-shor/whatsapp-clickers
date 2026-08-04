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
    // [ASSUMPTION]: EXPERIENCE.md specifies the confirm-stop dialog exists
    // but not its copy — flagged for Avraham to confirm/replace.
    stopConfirmTitle: 'לעצור את המשחק?',
    stopConfirmBody:
      'המשחק יסתיים ולא ניתן יהיה להמשיך אותו. משתתפים לא יקבלו הודעה על העצירה.',
    stopConfirmAction: 'עצור את המשחק',
    questionProgress: (n: number, total: number) => `שאלה ${n} מתוך ${total}`,
    gameOverTitle: 'המשחק הסתיים',
    actionError: 'הפעולה נכשלה. נסו שוב בעוד רגע.',
    // No refresh instruction (UX-DR: no refresh dependency on live web
    // surfaces) — the WS broadcast already carries the current state; this
    // banner self-clears once it arrives.
    actionConflict: 'מצב המשחק השתנה בינתיים. הלוח יתעדכן אוטומטית.',
    startGameNoQuestions: 'הוסיפו לפחות שאלה אחת כדי להתחיל את המשחק.',
  },
} as const
