const storageKey = "kvtide.demo-user-id";

export function getDemoUserID(): string {
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
