
const cookieParser = require('cookie-parser');
const express = require('express');

const blocklist = JSON.parse(process.env.BLOCKLIST);
const allowlist = JSON.parse(process.env.ALLOWLIST);

const app = express();
const port = process.env.PORT || 3001;
app.use(cookieParser());

app.get('/', (req, res) => {

  try {

    let filteredCookies = {}
    let cookieString = ""

    // Do the actual filtering on the cookies object keys, in
    // the presence of a blocklist, we ignore the allowlist
    if (blocklist != null) {
      filteredCookies = Object.keys(req.cookies)
        .filter(key => !blocklist.includes(key))
        .reduce((obj, key) => {
          obj[key] = req.cookies[key];
          return obj;
        }, {});
    } else if (allowlist != null) {
      filteredCookies = Object.keys(req.cookies)
        .filter(key => allowlist.includes(key))
        .reduce((obj, key) => {
          obj[key] = req.cookies[key];
          return obj;
        }, {});
    } else {
      filteredCookies = req.cookies
    }

    // Write cookies object into a semicolon-separated string
    if (Object.keys(filteredCookies).length > 0) {
      cookieString = Object.keys(filteredCookies)
        .map(key => `${key}=${req.cookies[key]}`)
        .reduce((i1, i2) => `${i1}; ${i2}`)
    }

    // Set Cookie header on response (will be picked up by traefik).
    res.set("Cookie", cookieString)
    res.sendStatus(200)

  } catch (err) {
    // We make sure that access is blocked if anything fails.
    console.error(err);
    res.sendStatus(503);
  }

})

app.listen(port, () => {
  console.log(`Cookie-cleaner listening at http://localhost:${port}`)
})
