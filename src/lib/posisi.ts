import { app } from './store.svelte';
import { surah } from './quran';
import type { JenisTarget, Setoran } from './types';

/** "Lanjut An-Naba' : 5" — dipakai di daftar supaya pengajar tahu posisi tanpa membuka santri. */
export function posisiLabel(santriId: string): string {
	const p = app.posisiZiyadah(santriId);
	return `Lanjut ${surah(p.surat).latin} : ${p.ayat}`;
}

/** Tahsin terakhir santri: surat dan ayat, plus level bila diisi. */
export function tahsinLabel(santriId: string): string {
	const z = app.setoranSantri(santriId).find((x) => x.jenis === 'tahsin');
	return z ? teksSetoran(z) : 'Belum ada';
}

/** Ringkasan satu setoran untuk riwayat dan ekspor. */
export function teksSetoran(z: Setoran): string {
	const bagian: string[] = [];
	if (z.suratAwal != null) {
		const a = z.ayatAwal === z.ayatAkhir ? `ayat ${z.ayatAwal}` : `ayat ${z.ayatAwal}–${z.ayatAkhir}`;
		bagian.push(`${surah(z.suratAwal).latin} · ${a}`);
	}
	if (z.jenis === 'tahsin') {
		const lv = app.levelById(z.levelId);
		if (lv) bagian.push(lv.nama);
		if (z.halamanAwal != null) {
			bagian.push(z.halamanAwal === z.halamanAkhir ? `hal. ${z.halamanAwal}` : `hal. ${z.halamanAwal}–${z.halamanAkhir}`);
		}
	}
	return bagian.length ? bagian.join(' · ') : z.jenis === 'tahsin' ? 'Tahsin' : '-';
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
	tahsin: 'Tahsin: memperbaiki bacaan surat dan ayat.'
} as const;

export const NAMA_TARGET: Record<JenisTarget, string> = {
	'ziyadah-pekan': 'Hafalan baru per pekan',
	hafalan: 'Capaian hafalan',
	tahsin: 'Naik level tahsin'
};

/** Pilihan target: pengajar cukup memilih jenis lalu mengisi angka. */
export const PILIHAN_TARGET: {
	id: 'juz' | 'surat' | 'level';
	nama: string;
	tanya: string;
	jenis: JenisTarget;
	satuan: string;
	max: number;
}[] = [
	{ id: 'juz', nama: 'Juz', tanya: 'Berapa juz?', jenis: 'hafalan', satuan: 'juz', max: 30 },
	{ id: 'surat', nama: 'Surat', tanya: 'Berapa surat?', jenis: 'hafalan', satuan: 'surat', max: 114 },
	{ id: 'level', nama: 'Naik level', tanya: 'Naik berapa level?', jenis: 'tahsin', satuan: 'level', max: 30 }
];
