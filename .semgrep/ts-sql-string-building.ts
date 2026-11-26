// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Test cases for .semgrep/ts-sql-string-building.yml. Run: semgrep --test .semgrep
// This file is not part of the web app build.

declare const db: {
  query(sql: string, values?: unknown[]): Promise<unknown>;
  execute(sql: string, values?: unknown[]): Promise<unknown>;
};

export async function bad(name: string, column: string): Promise<void> {
  // ruleid: kolikaja-ts-sql-string-building
  await db.query(`SELECT id FROM users WHERE display_name = '${name}'`);

  // ruleid: kolikaja-ts-sql-string-building
  await db.execute("DELETE FROM users WHERE display_name = '" + name);

  // ruleid: kolikaja-ts-sql-string-building
  const q = `SELECT id FROM users ORDER BY ${column}`;
  await db.query(q);
}

export async function good(name: string): Promise<void> {
  // ok: kolikaja-ts-sql-string-building
  await db.query("SELECT id FROM users WHERE display_name = $1", [name]);

  // ok: kolikaja-ts-sql-string-building
  await db.query(`SELECT id FROM users WHERE locale = 'hu'`);
}
