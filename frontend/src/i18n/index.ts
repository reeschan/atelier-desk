import { common } from "./ja/common";
import { flow } from "./ja/flow";
import { runs } from "./ja/runs";
import { settings } from "./ja/settings";
import { settings2 } from "./ja/settings-2";
import { skills } from "./ja/skills";

/** Every text of the screens, by namespace. The one place the words live. */
export const messages = { common, flow, skills, settings, settings2, runs } as const;

export type Namespace = keyof typeof messages;

/**
 * Looks a text up by namespace and key. `{name}` in the text is replaced by
 * vars.name. A key that is not there comes back as the key itself.
 */
export function t(ns: Namespace, key: string, vars?: Record<string, string | number>): string {
  const table = messages[ns] as Record<string, unknown>;
  const text = Object.prototype.hasOwnProperty.call(table, key) ? table[key] : undefined;
  if (typeof text !== "string") return key;
  if (vars === undefined) return text;
  return text.replace(/\{(\w+)\}/g, (whole, name: string) =>
    Object.prototype.hasOwnProperty.call(vars, name) ? String(vars[name]) : whole,
  );
}

/** The text of a DeskError code; a code nobody knows gets the `unknown` text. */
export function errorText(code: string): string {
  const errors = common.errors as Record<string, string>;
  return Object.prototype.hasOwnProperty.call(errors, code) ? errors[code] : errors.unknown;
}
