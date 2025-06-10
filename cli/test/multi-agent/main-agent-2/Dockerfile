FROM node:22-alpine

WORKDIR /blaxel/shared
COPY shared /blaxel/shared
RUN npx pnpm install

WORKDIR /blaxel/main-agent-2

COPY main-agent-2/package.json /blaxel/main-agent-2/
RUN npx pnpm install

COPY main-agent-2 .

RUN npx pnpm build
ENTRYPOINT ["npx", "pnpm", "start"]