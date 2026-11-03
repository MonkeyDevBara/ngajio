/** Foto profil: dipotong persegi di tengah, diperkecil, dan disimpan sebagai data URL JPEG kecil. */
export const FOTO_AWAL = 'data:image/jpeg;base64,';
const MAKS = 6000;

/** Hanya data URL JPEG hasil `kecilkanFoto` yang boleh dipakai sebagai src gambar. */
export const fotoAman = (f?: string): f is string => !!f && f.startsWith(FOTO_AWAL);

function muat(src: string): Promise<HTMLImageElement> {
	return new Promise((ok, gagal) => {
		const img = new Image();
		img.crossOrigin = 'anonymous';
		img.onload = () => ok(img);
		img.onerror = () => gagal(new Error('Gambar tidak bisa dibaca'));
		img.src = src;
	});
}

function jpeg(img: HTMLImageElement, sisi: number, mutu: number): string {
	const sisiAsal = Math.min(img.naturalWidth, img.naturalHeight);
	const c = document.createElement('canvas');
	c.width = c.height = sisi;
	const g = c.getContext('2d')!;
	g.fillStyle = '#fff'; // PNG transparan menjadi latar putih
	g.fillRect(0, 0, sisi, sisi);
	g.drawImage(
		img,
		(img.naturalWidth - sisiAsal) / 2,
		(img.naturalHeight - sisiAsal) / 2,
		sisiAsal,
		sisiAsal,
		0,
		0,
		sisi,
		sisi
	);
	return c.toDataURL('image/jpeg', mutu);
}

/** URL gambar (data:, blob:, atau https dengan CORS) masuk, data URL JPEG kecil keluar. */
export async function kecilkanFoto(src: string): Promise<string> {
	const img = await muat(src);
	for (const [sisi, mutu] of [[96, 0.6], [96, 0.45], [80, 0.45]]) {
		const d = jpeg(img, sisi, mutu);
		if (d.length <= MAKS && fotoAman(d)) return d;
	}
	throw new Error('Foto terlalu besar');
}

export async function fotoDariBerkas(f: File): Promise<string> {
	const url = URL.createObjectURL(f);
	try {
		return await kecilkanFoto(url);
	} finally {
		URL.revokeObjectURL(url);
	}
}
