import adapter from '@sveltejs/adapter-static';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	server: {
		// Satu origin dengan UI supaya cookie dan pemeriksaan Origin backend jalan.
		// changeOrigin false: header Host tetap host dev, sama dengan Origin browser.
		proxy: { '/api': { target: 'http://127.0.0.1:8787', changeOrigin: false } }
	},
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			adapter: adapter({ fallback: 'index.html' })
		})
	]
});
