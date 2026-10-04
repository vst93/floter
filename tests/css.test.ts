// R81 · the shared parser's own unit tests.
//
// The suites that consume `tests/css.ts` read real stylesheets; these two
// pin the parser's *contract* directly, so the shared file cannot drift while
// every consumer happens to pass. One test per behaviour the R81 report
// promises: comment stripping, and the one-brace-level at-rule walk.
import assert from "node:assert/strict";
import test from "node:test";

import { decl, ruleFor, rules, stripComments } from "./css.ts";

test("stripComments removes every comment, including multi-line ones", () => {
  assert.equal(stripComments("a/* one */b"), "ab");
  assert.equal(stripComments("a/*\n * two\n */b"), "ab");
  assert.equal(stripComments("a/* x */b/* y */c"), "abc");
  // A comment may sit where a selector would: without stripping, the rule
  // after it is unreachable, which is what the consuming suites depend on.
  assert.equal(stripComments("/* gone */ .a { color: red; }"), " .a { color: red; }");
});

test("rules reads one brace level: an at-rule is skipped and its inner rules are returned", () => {
  const css = "@media (min-width: 10px) {\n  .a,\n  .b   { color: red; }\n}";
  // The at-rule's own braces are not a rule; the nested rules are, with the
  // selector group's whitespace collapsed to single spaces.
  assert.deepEqual(rules(css), [{ selector: ".a, .b", body: " color: red; " }]);
  assert.equal(decl(rules(css)[0].body, "color"), "red");
  assert.equal(decl(rules(css)[0].body, "missing"), null);
  assert.equal(ruleFor(css, ".b"), " color: red; ");
});
