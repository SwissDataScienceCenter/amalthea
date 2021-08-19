FROM node:16.6.0

LABEL maintainer="info@datascience.ch"

WORKDIR /usr/src/app

COPY package*.json ./
COPY app.js ./

RUN npm ci --only=production

CMD [ "node", "app.js" ]
