FROM node:19-alpine as build

WORKDIR /app
COPY . .
RUN npm install
RUN npm run build

FROM node:19-alpine as deploy

WORKDIR /app
COPY package.json ./
COPY --from=build /app/build ./
COPY --from=build /app/node_modules ./node_modules
ADD entry.js ./

ENV PORT 8080
CMD ["node", "entry.js"]

