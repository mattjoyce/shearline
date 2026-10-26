---
type: Concept
title: Constants
description: "The engine's values: three scalar kinds, a UTF-8 string domain, exact decimal numbers with one canonical text and -0 folded to 0."
tags: [constants, kinds, utf8, numbers]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.3.1); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# Kinds

A `tarski.Const` is a string, a number or a boolean, the scalar kinds JSON has. No null, list or object. Kinds sort `KindString < KindNumber < KindBool`, and a constant's identity includes its kind, so the string `"1"` and the number `1` are different constants.

| Constructor | Notes |
|---|---|
| `String(s)` | Total; does not check UTF-8 itself |
| `Bool(b)` | |
| `Number(lexeme)` | JSON lexeme, read exactly; not a JSON number or not finite is `ErrSyntax` |
| `Int(n int64)` | Same constant `Number` gives for the decimal text; every int64 is exact |
| `Float(f float64)` | Same as `Number` for the value's text; NaN or Inf is `ErrInvalid` |

# String domain is valid UTF-8

JSON writes an invalid byte and U+FFFD as the same text, so two distinct facts would share an `Answer` and its hash. The two entry points refuse invalid UTF-8: the lexer, and `NewBase` / `Base.With`. `String` itself stays total, so a `Const` built there is checked where it enters a base. The lexer also reads runes properly, not bytes (module panel A9). This was edge audit finding 5. See [edge audit](/reviews/edge-audit-2026-10-22.md).

# No NaN, no Inf

`NaN` would equal everything under compare: `<` and `>` are both false, so the ordering returns zero. A consumer converting an inbound `"hour: NaN"` header could have handed the engine a constant that satisfies every equality. `Inf` has no JSON text. Both are refused at construction. See [declared context](/concepts/declared-context.md) for where such values enter.

# Negative zero is zero

`-0` is zero, with one sign, so identity and comparison agree: every constructor gives the same canonical lexeme.

# Numbers are exact decimals

Since tarski v0.1.0 a number is held as the decimal its lexeme spells, not as a float64: `9007199254740993` stays itself and does not collapse into its neighbour, and two numbers a float64 cannot tell apart compare as two. There is one canonical text, so `1` and `1.0` are the same constant and hash the same. The text is plain digits while the plain form stays within 64 digits and exact scientific form beyond that, so `1000000` and `0.00001` print in full, and a lexeme with a leading zero (`01`) is refused. An exponent beyond nine digits either way is `ErrSyntax`. `Float` goes through the shortest decimal that reads back. Keep names and ids as strings anyway: a number prints in canonical form, not as written.

# Identity keys

A tuple's key is the concatenation of its constants' keys, each with a kind byte and a length prefix, so it is self-delimiting. The old key was not injective over NUL: two different tuples could collide and one derived tuple could vanish, a wrong deny or a wrong `Permitted` listing (module panel A6). The fix wrote keys into one buffer, and `Const.same` compares in `bind` without building keys, which brought allocations below the original.

# JSON

`MarshalJSON` writes the value it came from; `UnmarshalJSON` reads it back, so an `Answer` decoded on a client has the same bytes and hash as the server's.

# Related

- [Proof](/concepts/proof.md) and the `Answer` hash depend on this identity.
- [Prefix grants](/concepts/prefix-grants.md): all grant positions are non-empty strings.

# Citations

[1] [DESIGN.md, Constants and string domain](../../docs/DESIGN.md)
[2] [tarski/term.go](https://github.com/mattjoyce/tarski/blob/v0.3.1/term.go)
[3] [Module panel collation](../../docs/reviews/luminary-module-2026-10-22/00-collation.md)
