import { app } from './store.svelte';
import { teksSetoran } from './posisi';

function unduh(nama: string, mime: string, isi: string) {
	const url = URL.createObjectURL(new Blob([isi], { type: mime }));
	const a = document.createElement('a');
	a.href = url;
	a.download = nama;
	document.body.appendChild(a);
	a.click();
	a.remove();
	setTimeout(() => URL.revokeObjectURL(url), 1000);
}

const sel = (v: unknown) => {
	const s = v == null ? '' : String(v);
	return /[",\n;]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
};
const baris = (...kolom: unknown[]) => kolom.map(sel).join(',');

export function eksporJSON() {
	const data = {
		aplikasi: 'Ngajio',
		versi: 1,
		diekspor: new Date().toISOString(),
		pengajar: app.pengajar,
		metode: app.metode,
		kelompok: app.kelompok,
		santri: app.santri,
		setoran: app.setoran,
		target: app.target
	};
	unduh('ngajio-data.json', 'application/json', JSON.stringify(data, null, 2));
	app.catatEkspor();
}

/** Tiga berkas CSV: santri, kelompok, setoran. Bisa dibuka ulang di Excel atau Sheets. */
export function eksporCSV() {
	const kel = (id: string) => app.kelompokById(id)?.nama ?? '';
	const santri = [
		baris('id', 'nama', 'kelompok', 'metode', 'arah_hafalan', 'wa_wali', 'arsip'),
		...app.santri.map((s) =>
			baris(s.id, s.nama, kel(s.kelompokId), app.metodeById(s.metodeId)?.nama, s.arah, s.waWali, s.arsip)
		)
	];
	const kelompok = [
		baris('id', 'nama', 'jadwal', 'metode', 'arsip'),
		...app.kelompok.map((k) => baris(k.id, k.nama, k.jadwal, app.metodeById(k.metodeId)?.nama, k.arsip))
	];
	const setoran = [
		baris('id', 'waktu', 'santri', 'kelompok', 'jenis', 'posisi', 'nilai', 'tag', 'catatan', 'dihapus'),
		...app.setoran.map((z) =>
			baris(
				z.id,
				z.dicatatPada,
				app.santriById(z.santriId)?.nama,
				kel(z.kelompokSaatItu),
				z.jenis,
				teksSetoran(z),
				z.nilai,
				z.tag.join('|'),
				z.catatan,
				z.hapusPada ? 'ya' : ''
			)
		)
	];
	const csv = 'text/csv;charset=utf-8';
	unduh('ngajio-santri.csv', csv, '﻿' + santri.join('\n'));
	setTimeout(() => unduh('ngajio-kelompok.csv', csv, '﻿' + kelompok.join('\n')), 300);
	setTimeout(() => unduh('ngajio-setoran.csv', csv, '﻿' + setoran.join('\n')), 600);
	app.catatEkspor();
}
