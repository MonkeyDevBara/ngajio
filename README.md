# Ngajio

## Coba lokal

```sh
npm install
npm run dev:all   # backend Go di 127.0.0.1:8787 dan UI di http://127.0.0.1:5173 (Ctrl-C menghentikan keduanya)
# buka http://127.0.0.1:5173/masuk lalu pakai tombol "Masuk dev (lokal)"
```

Butuh Go terpasang. Data lokal backend ada di `server/data/` (tidak masuk git). Jalankan terpisah dengan `npm run dev:api` dan `npm run dev -- --host 127.0.0.1 --port 5173`.

# sv

Everything you need to build a Svelte project, powered by [`sv`](https://github.com/sveltejs/cli).

## Creating a project

If you're seeing this, you've probably already done this step. Congrats!

```sh
# create a new project
npx sv create my-app
```

To recreate this project with the same configuration:

```sh
# recreate this project
npx sv@1.2.0 create --template minimal --types ts --install npm ngajio
```

## Adding features

Add features to your project with `sv add`:

```sh
npx sv add
```

For example, to add Tailwind CSS:

```sh
npx sv add tailwindcss
```

## Developing

Once you've created a project and installed dependencies with `npm install` (or `pnpm install` or `yarn`), start a development server:

```sh
npm run dev

# or start the server and open the app in a new browser tab
npm run dev -- --open
```

## Building

To create a production version of your app:

```sh
npm run build
```

You can preview the production build with `npm run preview`.

> To deploy your app, you may need to install an [adapter](https://svelte.dev/docs/kit/adapters) for your target environment.
# ngajio
