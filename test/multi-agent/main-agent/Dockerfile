FROM node:22-alpine

WORKDIR /blaxel/shared
COPY shared /blaxel/shared
RUN npx pnpm install

WORKDIR /blaxel/main-agent

COPY main-agent/package.json /blaxel/main-agent/
RUN npx pnpm install

COPY main-agent .

RUN npx pnpm build
ENTRYPOINT ["npx", "pnpm", "start"]