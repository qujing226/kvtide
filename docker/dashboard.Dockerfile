FROM node:24-alpine AS build

WORKDIR /app
COPY dashboard/package.json dashboard/package-lock.json ./

RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY dashboard/ ./
RUN npm run build

FROM node:24-alpine

WORKDIR /app
ENV NODE_ENV=production
ENV PORT=5174

COPY dashboard/server.mjs ./server.mjs
COPY dashboard/server/ ./server/
COPY --from=build /app/dist ./dist

USER node
EXPOSE 5174
CMD ["node", "server.mjs"]
