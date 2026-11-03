export type BarisNama = { nama: string; status: 'ok' | 'duplikat' | 'kosong' };

/**
 * Mengurai daftar nama tempelan (mis. dari WhatsApp). Nomor urut dan penanda
 * butir di depan nama dibuang. Duplikat dan baris kosong ditandai, bukan dibuang diam-diam.
 */
export function parseDaftarNama(teks: string, sudahAda: Set<string> = new Set()): BarisNama[] {
	const lihat = new Set<string>();
	const bagi = teks.includes('\n') ? /\r?\n|;/ : /\r?\n|;|,/;
	const out: BarisNama[] = [];
	for (const mentah of teks.split(bagi)) {
		if (!mentah.trim()) continue;
		const nama = mentah
			.replace(/^\s*(\d+\s*[.)\-:]|[-•*·])\s*/, '')
			.replace(/\s+/g, ' ')
			.trim()
			.replace(/(^|\s)(\p{Ll})/gu, (_, a: string, b: string) => a + b.toUpperCase());
		if (!nama) {
			out.push({ nama: '(kosong)', status: 'kosong' });
			continue;
		}
		const k = nama.toLowerCase();
		out.push({ nama, status: sudahAda.has(k) || lihat.has(k) ? 'duplikat' : 'ok' });
		lihat.add(k);
	}
	return out;
}
