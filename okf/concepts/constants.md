---
type: Concept
title: Constants
description: "The engine's values: three scalar kinds, a UTF-8 string domain, finite float64 numbers with -0 folded to 0, and no exact integers beyond 2^53."
tags: [constants, kinds, utf8, numbers]
timestamp: 2026-10-22T00:00:00Z
---

# Kinds

A `datalog.Const` is a string, a number or a boolean, the scalar kinds JSON has. No null, list or object. Kinds sort `KindString < KindNumber < KindBool`, and a constant's identity includes its kind, so the string `"1"` and the number `1` are different constants.

| Constructor | Notes |
|---|---|
| `String(s)` | Total; does not check UTF-8 itself |
| `Bool(b)` | |
| `Number(lexeme)` | JSON lexeme; not finite is `ErrSyntax` |
| `Int(n int64)` | Same constant `Number` gives for the decimal text |
| `Float(f float64)` | Same as `Number` for the value's text; NaN or Inf is `ErrInvalid` |

# String domain is valid UTF-8

JSON writes an invalid byte and U+FFFD as the same text, so two distinct facts would share an `Answer` and its hash. The two entry points refuse invalid UTF-8: the lexer, and `NewBase` / `Base.With`. `String` itself stays total, so a `Const` built there is checked where it enters a base. The lexer also reads runes properly, not bytes (module panel A9). This was edge audit finding 5. See [edge audit](/reviews/edge-audit-2026-10-22.md).

# No NaN, no Inf

`NaN` would equal everything under compare: `<` and `>` are both false, so the ordering returns zero. A consumer converting an inbound `"hour: NaN"` header could have handed the engine a constant that satisfies every equality. `Inf` has no JSON text. Both are refused at construction. See [declared context](/concepts/declared-context.md) for where such values enter.

# Negative zero is zero

`-0` is folded to `0` in one place (`number`), so identity and comparison agree: both constructors give the same canonical lexeme.

# Numbers are float64 identifiers, not exact integers

An integer beyond 2^53 is held as the nearest float64 that fits, so it is not an exact identifier. Use a string for one. `1` and `1.0` are the same constant and hash the same.

# Identity keys

A tuple's key is the concatenation of its constants' keys, each with a kind byte and a length prefix, so it is self-delimiting. The old key was not injective over NUL: two different tuples could collide and one derived tuple could vanish, a wrong deny or a wrong `Permitted` listing (module panel A6). The fix wrote keys into one buffer, and `Const.same` compares in `bind` without building keys, which brought allocations below the original.

# JSON

`MarshalJSON` writes the value it came from; `UnmarshalJSON` reads it back, so an `Answer` decoded on a client has the same bytes and hash as the server's.

# Related

- [Proof](/concepts/proof.md) and the `Answer` hash depend on this identity.
- [Prefix grants](/concepts/prefix-grants.md): all grant positions are non-empty strings.

# Citations

[1] [DESIGN.md, Constants and string domain](../../docs/DESIGN.md)
[2] [datalog/term.go](../../datalog/term.go)
[3] [Module panel collation](../../docs/reviews/luminary-module-2026-10-22/00-collation.md)
