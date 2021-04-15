const express = require('express')
const app = express()
const port = 3000

const userId = process.env.USER_ID;

// Note: This is a really just a poc here which could go in two directions.
// Either get rid of this completely and let the oauth2 proxy handle it,
// potentially we'd have to contribute some functionality upstream. Or we
// see this as our entrypoint for enforcing rules which are configurable through
// the custom resource spec.

app.get('/', (req, res) => {
  console.log(req.headers)
  if (req.headers['x-auth-request-user'] === userId) {
    res.sendStatus(200)
  }
  res.sendStatus(404)
})

app.listen(port, () => {
  console.log(`Autorization plugin listening at http://localhost:${port}`)
})
