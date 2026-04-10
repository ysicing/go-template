import fs from "node:fs";
import path from "node:path";

import { describe, expect, it } from "vitest";

const sourceRoot = path.resolve(__dirname, "../..");
const sourceExtensions = new Set([".ts", ".tsx"]);
const parentImportPattern = /(?:from\s*["']\.\.\/|import\s*["']\.\.\/)/;

function collectSourceFiles(dir: string): string[] {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      return collectSourceFiles(fullPath);
    }
    if (!sourceExtensions.has(path.extname(entry.name))) {
      return [];
    }
    return [fullPath];
  });
}

describe("import style", () => {
  it("does not use parent relative imports in src", () => {
    const violations = collectSourceFiles(sourceRoot)
      .filter((filePath) => filePath !== __filename)
      .map((filePath) => {
        const content = fs.readFileSync(filePath, "utf8");
        return parentImportPattern.test(content) ? path.relative(sourceRoot, filePath) : null;
      })
      .filter((value): value is string => value !== null);

    expect(violations).toEqual([]);
  });
});
