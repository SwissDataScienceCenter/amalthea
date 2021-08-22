const express = require('express')
const app = express()
const port = 3000

const userId = process.env.USER_ID;

app.get('/', (req, res) => {
  if (req.headers['x-auth-request-user'] === userId) {
    res.sendStatus(200)
  }
  res.sendStatus(404)
})

app.listen(port, () => {
  console.log(`Autorization plugin listening at http://localhost:${port}`)
})
