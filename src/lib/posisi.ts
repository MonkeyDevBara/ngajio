import { app } from './store.svelte';
import { surah } from './quran';
import type { JenisTarget, Setoran } from './types';

/** "Lanjut An-Naba' : 5" — dipakai di daftar supaya pengajar tahu posisi tanpa membuka santri. */
export function posisiLabel(santriId: string): string {
	const p = app.posisiZiyadah(santriId);
	return `Lanjut ${surah(p.surat).latin} : ${p.ayat}`;
}

export function tahsinLabel(santriId: string): string {
	const p = app.posisiTahsin(santriId);
	const lv = app.levelById(p.levelId);
	return `${lv?.nama ?? 'Tahsin'} · hal. ${p.halaman}`;
}

/** Ringkasan satu setoran untuk riwayat dan ekspor. */
export function teksSetoran(z: Setoran): string {
	if (z.jenis === 'tahsin') {
		const lv = app.levelById(z.levelId);
		const h = z.halamanAwal === z.halamanAkhir ? `hal. ${z.halamanAwal}` : `hal. ${z.halamanAwal}–${z.halamanAkhir}`;
		return `${lv?.nama ?? 'Tahsin'} · ${h}`;
	}
	if (z.suratAwal == null) return '-';
	const a = z.ayatAwal === z.ayatAkhir ? `ayat ${z.ayatAwal}` : `ayat ${z.ayatAwal}–${z.ayatAkhir}`;
	return `${surah(z.suratAwal).latin} · ${a}`;
}

export const NAMA_JENIS = {
	ziyadah: 'Hafalan baru',
	murojaah: 'Murajaah',
	tahsin: 'Tahsin'
} as const;

export const TAG_KESALAHAN = {
	ziyadah: ['Makhraj', 'Mad', 'Lupa', 'Tajwid', 'Waqaf'],
	murojaah: ['Makhraj', 'Lupa', 'Tajwid', 'Terputus'],
	tahsin: ['Makhraj', 'Mad', 'Idgam', 'Waqaf', 'Tajwid']
} as const;

export const PENJELASAN_JENIS = {
	ziyadah: 'Ziyadah: setoran hafalan baru.',
	murojaah: 'Murajaah: mengulang hafalan yang sudah ada.',
	tahsin: 'Tahsin: memperbaiki bacaan per jilid dan halaman.'
} as const;

export const NAMA_TARGET: Record<JenisTarget, string> = {
	'ziyadah-pekan': 'Hafalan baru per pekan',
	hafalan: 'Capaian hafalan',
	tahsin: 'Naik jilid tahsin'
};

/** Template target (F-18): sekali ketuk membuat target lengkap. */
export const TEMPLATE_TARGET: {
	id: string;
	judul: string;
	ket: string;
	jenis: JenisTarget;
	nilai: number;
	satuan: string;
}[] = [
	{ id: 'juz30', judul: 'Juz 30 pemula', ket: 'Hafal 5 surat pendek', jenis: 'hafalan', nilai: 5, satuan: 'surat' },
	{ id: 'pilihan', judul: 'Surat pilihan', ket: 'Hafal 3 surat', jenis: 'hafalan', nilai: 3, satuan: 'surat' },
	{ id: 'juz', judul: 'Per juz', ket: 'Hafal 1 juz', jenis: 'hafalan', nilai: 1, satuan: 'juz' },
	{ id: 'pekan', judul: 'Rutin per pekan', ket: '3 setoran hafalan baru per pekan', jenis: 'ziyadah-pekan', nilai: 3, satuan: 'setoran/pekan' },
	{ id: 'jilid', judul: 'Naik jilid', ket: 'Naik 1 jilid tahsin', jenis: 'tahsin', nilai: 1, satuan: 'jilid' }
];
