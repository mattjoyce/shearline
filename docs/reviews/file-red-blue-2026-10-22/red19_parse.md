# Independent RED TEAM review: datalog/parse.go

Snapshot `5fe3a78`. I reviewed Parse, statement/literal/atom/term parsing, errors,
position/source metadata, and Program.Clone. Adjacent lexer, shape validation and
Compile behavior were checked only for parser seams. No production edits.

## Confirmed finding

### Low: repeated declarations silently overwrite earlier arity

Location: lines 155 (`p.prog.Declared[name] = arity`).

`Parse("p/1. p/2.")` returns success with only `p/2` in the declaration map;
reversing them returns only `p/1`. A declaration's earlier arity is lost before
Compile can detect a conflict. A later rule compatible with the final declaration
then compiles, so the accepted program does not represent all declaration statements
in its source.

Reproduced by the pre-existing focused probe `TestRed19ParserEvidence` in
`datalog/red19_parse_test.go`; output shows both sources parse successfully with
only the last arity retained. Whether repeated identical declarations should be
accepted is a language-design choice; conflicting declarations should either be
rejected here or preserved for Compile to diagnose. Severity is low because this
needs duplicated declarations in authored policy text and the later one wins
deterministically; no runtime memory-safety or direct authorization bypass was
demonstrated.

## Defenses and other observations

- Tokenization completes before parsing; syntax errors return the zero Program,
  not partially accumulated rules/declarations.
- `statement` checks `i+1` safely because the lexer provides an EOF token and the
  loop does not call it at EOF. `take` leaves EOF pinned, preventing runaway index
  advancement.
- Atom parsing requires at least one argument; declarations require positive
  integer arity; `strconv.Atoi` overflow is rejected. Compile/shape validation
  separately enforces relation consistency and safety.
- `not` is reserved in literal position, so a relation named `not` cannot be
  expressed as an atom. This follows the grammar and was not treated as a defect.
- Boolean constants cannot be used as comparison left terms because `true` and
  `false` lex as names and literal dispatch takes the atom branch. This is a
  language ambiguity/limitation: `p(X) :- true = false.` fails to parse, while
  numeric and string comparisons work. No documentation promises comparisons over
  booleans; this merits explicit grammar documentation if intended to be supported.
- Numeric syntax follows the lexer's accepted number tokens and Number conversion;
  number-lexeme strictness is covered in a separate file review. No parser panic,
  infinite loop, or partial AST escape was found on focused malformed cases.
- Clone copies the declaration map, rule slices, atom argument slices, and
  comparison pointers; strings and scalar terms need no deep copy. No alias from
  the original Program into Clone was found.

Verification: `go test ./datalog -run TestRed19ParserEvidence -v -count=1` passed.
