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
    emptyState: 'עוד אין כאן משחקים. יצירת משחקים תגיע בשלב הבא.',
    logout: 'התנתקות',
    logoutError: 'ההתנתקות מהשרת נכשלה. נסו שוב בעוד רגע.',
  },
  notFound: {
    message: 'הדף הזה לא קיים. ייתכן שהקישור שגוי או שהעמוד הוסר.',
    backHome: 'חזרה למסך הראשי',
  },
} as const
