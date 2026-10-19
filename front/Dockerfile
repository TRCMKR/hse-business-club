# Build stage
FROM node:22-alpine AS build
WORKDIR /app

COPY package.json package-lock.json ./
COPY lib ./lib

# postinstall (nuxt prepare) runs later with the full sources
RUN npm ci --ignore-scripts

COPY . ./

RUN npm run build

# Runtime stage: only the self-contained Nitro output is needed
FROM node:22-alpine
WORKDIR /app

COPY --from=build /app/.output/ ./

ENV NODE_ENV=production
ENV PORT=3000
ENV HOST=0.0.0.0
# Overridable at runtime:
# NUXT_API_BASE_URL=http://back:8080
# NUXT_MOCK_API=false

EXPOSE 3000

USER node

CMD ["node", "/app/server/index.mjs"]
