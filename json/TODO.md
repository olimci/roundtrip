# TODO: JSON

## Now

Nothing active.

## Definite

- [ ] cooler struct tags (can we do hex numbers/indent etc?)
- [ ] Array/Object re-ordering. Likely best options are a Swap, and a sortfunc. try to mirror slices package. (Will be pretty slow regardless but eh)
- [ ] Let Valid/Comact etc define syntax rules.
- [ ] Add Lexer tests.

## Parity

- [ ] Check streaming decoder behaviour matches json.

## Potential

- [ ] Is structural diffing something useful for this to offer? Is there an ergonmic way to do it.
  - Equally, is this something that would want to integrate with patch/merge patch.
- [ ] is it worth adding a cmd/? could hold structual diffs, some jq-like tool etc.
- [ ] standard comment tag format.
- [ ] merge left/right brace/bracket into one delim token
- [ ] Better fuzzing coverage.

## Writing

- [ ] Documentation
  - [x] README.md
  - [ ] Comments
- [ ] Examples
- [ ] Blogpost on the project
