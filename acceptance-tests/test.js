var assert = require('assert');
const util = require('util');
const exec = util.promisify(require('child_process').exec);
const axios = require('axios').default;

const token = "testtoken123456";
const host = "localhost";
const k8sNamespace = process.env.K8S_NAMESPACE || "default";
const image = process.env.TEST_IMAGE_NAME || "renku/renkulab-py:3.8-0.8.0";
const testSpec = process.env.TEST_SPEC || "jupyterlab.spec.js";
const env = process.env.ENVIRONMENT || "lab"
const sessionName = "test";
const timeoutSeconds = process.env.TIMEOUT_SECS || 600;

const url = `http://${host}/${sessionName}/${env}?token=${token}`
const manifest = `apiVersion: amalthea.dev/v1alpha1
kind: JupyterServer
metadata:
  name: ${sessionName}
  namespace: ${k8sNamespace}
spec:
  jupyterServer:
    image: ${image}
  routing:
    host: ${host}
    path: /${sessionName}
  auth:
    token: ${token}
`


function sleep(ms) {
  // shamelessly ripped off from https://stackoverflow.com/questions/951021/what-is-the-javascript-version-of-sleep
  return new Promise(resolve => setTimeout(resolve, ms));
}

const checkStatusCode = async function (url) {
  var count = 0;
  while (true) {
    console.log("Waiting for container to become ready...")
    try {
      res = await axios.get(url)
      if (res.status < 300) {
        return {"status": res.status};
      }
    }
    catch (err) {}
    finally {
      if (count > timeoutSeconds) {
        return {"error": "Timed out waiting for container to become ready"}
      }
      await sleep(1000);
      count = count + 1;
    }
  }
}


describe(`Starting session ${sessionName} with image ${image}`, function () {
  this.timeout(0);
  before(async function () {
    await exec(`cat <<EOF | kubectl apply -f - 
${manifest}
EOF`);
    const {status} = await checkStatusCode(url);
    assert(status < 300)
  });
  it('Should pass all acceptance tests', async function () {
    const {stdout, stderr, error} = await exec(`npx cypress run --spec cypress/integration/${testSpec} --env URL=${url}`);
    console.log(`\n\n--------------------------------------------Cypress stdout--------------------------------------------\n${stdout}`)
    console.log(`\n\n--------------------------------------------Cypress stderr--------------------------------------------\n${stderr}`)
    console.log(`\n\n--------------------------------------------Cypress error--------------------------------------------\n${error}`)
    console.log(`\n\n-----------------------------------------------------------------------------------------------------\n`)
    assert(!error)
  });
  after(async function () {
    console.log(`Stopping session with image ${image}.`)
    await exec(`cat <<EOF | kubectl delete -f - 
${manifest}
EOF`);
    console.log(`Container successfully stopped`)
  });
});
