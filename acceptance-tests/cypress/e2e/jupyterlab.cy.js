import { basicJupyterTests } from "@renku/notebooks-cypress-tests";

basicJupyterTests(Cypress.env("URL"))
