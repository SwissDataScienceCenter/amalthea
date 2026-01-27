var assert = require('assert');
const util = require('util');
const exec = util.promisify(require('child_process').exec);
const axios = require('axios').default;

const dns = require('node:dns');
dns.setDefaultResultOrder('ipv4first');

const token = "testtoken123456";
const hostName = process.env.HOSTNAME || process.env.HOST || "localhost";
const k8sNamespace = process.env.K8S_NAMESPACE || "default";
const image = process.env.TEST_IMAGE_NAME || "jupyter/base-notebook:latest";
const testSpec = process.env.TEST_SPEC || "jupyterlab.cy.js";
const env = process.env.ENVIRONMENT || "lab"
const sessionName = "test";
const timeoutSeconds = process.env.TIMEOUT_SECS || 600;

const url = `http://${hostName}/${env}?token=${token}`
const manifest = `---
apiVersion: v1
kind: Secret
metadata:
  name: ${sessionName}-secret
  namespace: ${k8sNamespace}
stringData:
  token: ${token}
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: ${sessionName}-cm
data:
  jupyter_server_config.py: |
    import os
    c.IdentityProvider.token=os.environ["SERVER_APP_TOKEN"]
    c.ServerApp.ip="0.0.0.0"
---
apiVersion: amalthea.dev/v1alpha1
kind: AmaltheaSession
metadata:
  name: ${sessionName}
  namespace: ${k8sNamespace}
spec:
  session:
    image: ${image}
    port: 8888
    env:
      - name: SERVER_APP_TOKEN
        valueFrom:
          secretKeyRef:
            name: ${sessionName}-secret
            key: token
    extraVolumeMounts:
      - name: test-cm
        mountPath: /etc/jupyter
  extraVolumes:
    - name: test-cm
      configMap:
        name: test-cm
  ingress:
    host: ${hostName}
    ingressClassName: "nginx"
`


function sleep(ms) {
  // shamelessly ripped off from https://stackoverflow.com/questions/951021/what-is-the-javascript-version-of-sleep
  return new Promise(resolve => setTimeout(resolve, ms));
}

const checkStatusCode = async function (url) {
  var count = 0;
  while (true) {
    console.log(`Waiting for container to become ready at ${url}...`)
    try {
      res = await axios.get(url)
      if (res.status < 300) {
        console.log(`Response from starting container succeeded with status code: ${res.status}`)
        return {"status": res.status};
      }
    }
    catch (err) {
      console.log(`Waiting to start for a container failed with error: ${err}.`)
    }
    finally {
      if (count > timeoutSeconds / 10) {
        console.log("Waiting for container to become available timed out.")
        return {"error": "Timed out waiting for container to become ready"}
      }
      await sleep(10000);
      count = count + 1;
    }
  }
}


describe(`Starting session ${sessionName} with image ${image}`, function () {
  this.timeout(0);
  before(async function () {
    console.log(`Launching session with manifest:\n${manifest}`)
    try {
      const {error} = await exec(`cat <<EOF | kubectl apply -f - 
${manifest}
EOF`);
      if (error) {
        console.log(`Error applying server manifest: ${error}`)
      }
    }
    catch (err) {
      console.log(`Error applying server manifest: ${err}`)
    }
    const {status} = await checkStatusCode(url);
    assert(status < 300 || status == 302)
  });
  it('Should pass all acceptance tests', async function () {
    console.log("Starting cypress tests")
    const {stdout, stderr, error} = await exec(`npx cypress run --spec cypress/e2e/${testSpec} --env URL=${url}`);
    console.log(`\n\n--------------------------------------------Cypress stdout--------------------------------------------\n${stdout}`)
    console.log(`\n\n--------------------------------------------Cypress stderr--------------------------------------------\n${stderr}`)
    console.log(`\n\n-----------------------------------------------------------------------------------------------------\n`)
    assert(!error, `Tests failed with error:\n${error}`)
  });
  after(async function () {
    console.log(`Stopping session with image ${image}.`)
    await exec(`cat <<EOF | kubectl delete -f - 
${manifest}
EOF`);
    console.log(`Container successfully stopped`)
  });
});
