FROM node:16.14.0

WORKDIR /decode/

COPY . .

RUN yarn

EXPOSE 8000

ENTRYPOINT node decode.js