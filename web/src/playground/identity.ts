const storageKey = "mini-llm-serve.playground-user-id";

export function getPlaygroundUserID(): string {
  const existing = localStorage.getItem(storageKey);
  if (existing) {
    return existing;
  }

  const suffix =
    typeof crypto.randomUUID === "function"
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const userID = `web-${suffix}`;
  localStorage.setItem(storageKey, userID);
  return userID;
}
