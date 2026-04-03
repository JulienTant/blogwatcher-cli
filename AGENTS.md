### While developping

- You must never ignore an error. If this this a production error, log it. If it is a test error, assert/require on it.
- Tests will use the assertify library, with `assert` and `require`. Do not use `t.` functions to do assertion.


### After developping

- Ensure your files are correctly formatted using `goimport -w -l`