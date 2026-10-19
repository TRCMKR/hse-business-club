# Build Stage 1

FROM node:22-alpine AS build
WORKDIR /app

RUN corepack enable

COPY package.json package-lock.json ./

COPY lib ./lib

# Install dependencies
RUN npm i

# Copy the entire project
COPY . ./

# Build the project
RUN npm run build

# Build Stage 2

FROM node:22-alpine
WORKDIR /app

# Only `.output` folder is needed from the build stage
COPY --from=build /app/.output/ ./

# Change the port and host
ENV PORT=8080
ENV HOST=0.0.0.0

EXPOSE 8080

CMD ["node", "/app/server/index.mjs"]
