var assert = require('assert');
const util = require('util');
const exec = util.promisify(require('child_process').exec);
const axios = require('axios').default;

const token = "testtoken123456";
const host = "localhost";
const k8sNamespace = process.env.K8S_NAMESPACE || "default";
const testSpecs = [
  {
    "image": "renku/renkulab-r",
    "sessionName": "r",
    "testSpec": "jupyterlab.spec.js",
    "tag": "4.0.5-0.8.0",
    "env": "lab"
  },
  {
    "image": "renku/renkulab-cuda-tf",
    "sessionName": "cuda-tf",
    "testSpec": "jupyterlab.spec.js",
    "tag": "0.8.0",
    "env": "lab"
  },
  {
    "image": "renku/renkulab-julia",
    "sessionName": "julia",
    "testSpec": "jupyterlab.spec.js",
    "tag": "1.6.1-0.8.0",
    "env": "lab"
  },
  {
    "image": "renku/renkulab-py",
    "sessionName": "py",
    "testSpec": "jupyterlab.spec.js",
    "tag": "3.8-0.8.0",
    "env": "lab"
  },
]


function sleep(ms) {
  // shamelessly ripped off from https://stackoverflow.com/questions/951021/what-is-the-javascript-version-of-sleep
  return new Promise(resolve => setTimeout(resolve, ms));
}

const checkStatusCode = async function (url) {
  var count = 0;
  const maxCount = 300;  // sleeps 1 sec per iteration
  while (true) {
    try {
      res = await axios.get(url)
      if (res.status < 300 || count > maxCount) {
        return {"status": res.status};
      }
      count = count + 1;
      await sleep(1000);
    }
    catch (error) {
      icount = count + 1;
      await sleep(1000);
    }
  }
}

testSpecs.forEach(function(spec, _) {
  var url = `http://${host}/${spec.sessionName}/${spec.env}?token=${token}`
  var manifest = `apiVersion: renku.io/v1alpha1
kind: JupyterServer
metadata:
  name: ${spec.sessionName}
  namespace: ${k8sNamespace}
spec:
  jupyterServer:
    image: ${spec.image}:${spec.tag}
  routing:
    host: ${host}
    path: /${spec.sessionName}
  auth:
    token: ${token}`
  describe(`Starting session ${spec.sessionName} with image ${spec.image}:${spec.tag}`, function () {
    this.timeout(0);
    before(async function () {
      await exec(`cat <<EOF | kubectl apply -f - 
${manifest}
EOF`);
      const {status} = await checkStatusCode(url);
      assert(status < 300)
    });
    it('Should pass all acceptance tests', async function () {
      const {stdout, stderr, error} = await exec(`npx cypress run --spec cypress/integration/${spec.testSpec} --env URL=${url}`);
      console.log(`\n\n--------------------------------------------Cypress stdout--------------------------------------------\n${stdout}`)
      console.log(`\n\n--------------------------------------------Cypress stderr--------------------------------------------\n${stderr}`)
      console.log(`\n\n--------------------------------------------Cypress error--------------------------------------------\n${error}`)
      console.log(`\n\n-----------------------------------------------------------------------------------------------------\n`)
      assert(!error)
    });
    after(async function () {
      console.log(`Stopping session with image ${spec.image}:${spec.tag}.`)
      await exec(`cat <<EOF | kubectl delete -f - 
${manifest}
EOF`);
      console.log(`Container successfully stopped`)
    });
  });
})
