import { beforeEach, describe, expect, it, vi } from "vitest";

import { getDemoUserID } from "./identity";

describe("getDemoUserID", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("keeps one user ID for repeated requests in the same browser", () => {
    vi.spyOn(crypto, "randomUUID").mockReturnValue(
      "11111111-1111-4111-8111-111111111111",
    );

    const first = getDemoUserID();
    const second = getDemoUserID();

    expect(first).toBe("web-11111111-1111-4111-8111-111111111111");
    expect(second).toBe(first);
  });
});
