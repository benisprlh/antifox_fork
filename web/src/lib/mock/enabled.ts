// Public build-time mock toggle; NEXT_PUBLIC_ makes it available in browsers.
// NEXT_PUBLIC_MOCK=1 enables the entire backend-free demo.
export const MOCK = process.env.NEXT_PUBLIC_MOCK === "1";
