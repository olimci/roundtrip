# TODO: JSON

## Parity

- [x] Better JSON parity: 
  - [x] Util funcs like Valid, Compact, Indent, HTMLEscape. Compact/Indent preseerving comments is an obvious use case.
  - [x] Better encoding/decoding option ergonomics
  - [x] Repeated encodes/decodes on one encoder/decoder. Perhaps we can detatch the linked list collector from the lexer, since that is likely the main issue. (actually that feels like a better seperation anyway)
  - [x] Improve struct field resolution, to parity with the standard library (+Anonymous struct field resolution)
  - [x] Check if omitempty/omitzero are good as is. (might be fine)
  - [x] Disallow unknown fields option.

## Features

- [ ] Node accessors
  - [ ] Some sort of simple path accessor
  - [ ] JSON pointer accessor
- [ ] Better Comment APIs
- [ ] Object/array field tools
  - [ ] Moving/positioning things.

## Potential Issues

- [ ] Marshal cycle detection

## Longterm

- Is structural diffing something useful for this to offer? Is there an ergonmic way to do it.
- Documentation/Examples
- Blogpost on the project
