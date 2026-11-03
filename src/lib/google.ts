import { api, type BalasanAuth } from './sinkron';

interface GoogleId {
	initialize(o: { client_id: string; callback: (r: { credential?: string }) => void }): void;
	renderButton(el: HTMLElement, o: Record<string, unknown>): void;
}
type GoogleWindow = Window & { google?: { accounts: { id: GoogleId } } };

let muatan: Promise<void> | null = null;

/** Memuat skrip Google Identity Services sekali saja. Hanya dipanggil dari halaman masuk. */
function muatSkrip(): Promise<void> {
	if ((window as GoogleWindow).google?.accounts?.id) return Promise.resolve();
	muatan ??= new Promise<void>((ok, gagal) => {
		const s = document.createElement('script');
		s.src = 'https://accounts.google.com/gsi/client';
		s.async = true;
		s.onload = () => ok();
		s.onerror = () => {
			muatan = null;
			s.remove();
			gagal(new Error('Skrip Google tidak bisa dimuat. Periksa sinyal Anda.'));
		};
		document.head.appendChild(s);
	});
	return muatan;
}

/** Menggambar tombol Google di `el`. Callback mengirim {credential} ke /api/auth/google. */
export async function pasangGoogle(
	el: HTMLElement,
	clientId: string,
	onMasuk: (r: BalasanAuth) => void,
	onGalat: (e: unknown) => void
) {
	try {
		await muatSkrip();
		const id = (window as GoogleWindow).google!.accounts.id;
		id.initialize({
			client_id: clientId,
			callback: async ({ credential }) => {
				if (!credential) return;
				try {
					onMasuk(await api<BalasanAuth>('/api/auth/google', { credential }));
				} catch (e) {
					onGalat(e);
				}
			}
		});
		id.renderButton(el, {
			theme: 'outline',
			size: 'large',
			text: 'continue_with',
			shape: 'pill',
			locale: 'id',
			width: Math.min(400, el.clientWidth || 320)
		});
	} catch (e) {
		onGalat(e);
	}
}
