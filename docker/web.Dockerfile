FROM node:24-alpine AS build

WORKDIR /app
COPY web/package.json web/package-lock.json ./

RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY web/ ./
RUN npm run build

FROM node:24-alpine

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=5173

COPY web/server.mjs ./server.mjs
COPY web/server/ ./server/
COPY --from=build /app/dist ./dist

USER node
EXPOSE 5173
CMD ["node", "server.mjs"]
