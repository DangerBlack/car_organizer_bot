FROM node:14.17-alpine

RUN apk --no-cache add --virtual .gyp make gcc g++ python git

COPY .env package-lock.json package.json ./

RUN npm i

COPY src/ ./src/

ENV NODE_ENV=production

EXPOSE 3000

CMD ["npm", "run", "start"]
