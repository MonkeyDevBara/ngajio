export const HARI = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu'];
export const BULAN = [
	'Januari',
	'Februari',
	'Maret',
	'April',
	'Mei',
	'Juni',
	'Juli',
	'Agustus',
	'September',
	'Oktober',
	'November',
	'Desember'
];
export const BULAN_SINGKAT = BULAN.map((b) => b.slice(0, 3));

const TZ = 'Asia/Jakarta';

export function tanggal(iso: string, opts: { hari?: boolean; tahun?: boolean; jam?: boolean } = {}) {
	const d = new Date(iso);
	return new Intl.DateTimeFormat('id-ID', {
		timeZone: TZ,
		weekday: opts.hari ? 'long' : undefined,
		day: 'numeric',
		month: 'long',
		year: opts.tahun ? 'numeric' : undefined,
		hour: opts.jam ? '2-digit' : undefined,
		minute: opts.jam ? '2-digit' : undefined,
		hour12: false
	}).format(d);
}

export function jam(iso: string) {
	return new Intl.DateTimeFormat('id-ID', {
		timeZone: TZ,
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	}).format(new Date(iso));
}

/** Kunci hari dalam zona Asia/Jakarta, format YYYY-MM-DD. */
export function hariKey(d: Date | string): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone: TZ,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(typeof d === 'string' ? new Date(d) : d);
}

export function inRange(iso: string, from: Date, to: Date) {
	const t = new Date(iso).getTime();
	return t >= from.getTime() && t <= to.getTime();
}

export function daysAgo(iso: string, ref = new Date()): number {
	const a = hariKey(iso);
	const b = hariKey(ref);
	return Math.round((new Date(b).getTime() - new Date(a).getTime()) / 86400000);
}

/** "Hari ini", "Kemarin", "3 hari lalu", atau tanggal lengkap. */
export function relatif(iso: string, ref = new Date()): string {
	const d = daysAgo(iso, ref);
	if (d <= 0) return 'Hari ini';
	if (d === 1) return 'Kemarin';
	if (d < 7) return `${d} hari lalu`;
	return tanggal(iso);
}

export function inisial(nama: string) {
	return nama
		.split(' ')
		.filter((w) => w.length > 1)
		.slice(0, 2)
		.map((w) => w[0])
		.join('')
		.toUpperCase();
}

/** Warna avatar deterministik dari nama, supaya tiap santri terasa "punya warna". */
const AVATAR = [
	['#0d6b53', '#e3f0ea'],
	['#8a5a10', '#f7eed6'],
	['#31527a', '#e6eef7'],
	['#7a3155', '#f6e7ef'],
	['#4a5f22', '#eef3e0'],
	['#7a3a31', '#f7e8e5']
];

export function avatarWarna(nama: string) {
	let h = 0;
	for (let i = 0; i < nama.length; i++) h = (h * 31 + nama.charCodeAt(i)) % 997;
	return AVATAR[h % AVATAR.length];
}

import type { SkalaJenis } from './types.ts';

/** Nilai disimpan 1–5. Skala 'angka' menampilkannya sebagai 20, 40, 60, 80, 100. */
export function nilaiLabel(n: number, skala: SkalaJenis): string {
	if (skala === 'angka') return n >= 1 && n <= 5 ? String(n * 20) : '-';
	if (skala === 'huruf') return ['E', 'D', 'C', 'B', 'A'][n - 1] ?? '-';
	if (skala === 'bintang') return '★'.repeat(n) + '☆'.repeat(5 - n);
	return ['Kurang', 'Cukup', 'Baik', 'Baik Sekali', 'Mumtaz'][n - 1] ?? '-';
}

export function angka(n: number, digit = 0) {
	return new Intl.NumberFormat('id-ID', {
		minimumFractionDigits: digit,
		maximumFractionDigits: digit
	}).format(n);
}

export function rupiah(n: number) {
	return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(n);
}
