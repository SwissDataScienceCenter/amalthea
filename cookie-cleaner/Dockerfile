FROM node:14

LABEL maintainer="info@datascience.ch"

WORKDIR /usr/src/app

COPY package*.json ./
COPY app.js ./

RUN npm ci --only=production

CMD [ "node", "app.js" ]
