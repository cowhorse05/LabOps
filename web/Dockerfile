FROM node:22-alpine

WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install && chmod -R +x node_modules/.bin
COPY . .

EXPOSE 5173

CMD ["node", "./node_modules/vite/bin/vite.js", "--host", "0.0.0.0"]
