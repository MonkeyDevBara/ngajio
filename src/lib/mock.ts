import { SURAHS, JUZ30_REVERSE, surah } from './quran';
import type {
	Arah,
	Jenis,
	Kelompok,
	Metode,
	Pengajar,
	Santri,
	Setoran,
	Target
} from './types';

/** PRNG deterministik supaya angka laporan stabil tiap reload. */
function mulberry32(seed: number) {
	return function () {
		seed |= 0;
		seed = (seed + 0x6d2b79f5) | 0;
		let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
		t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
}

/** Tanggal "hari ini" versi mock. Development dimulai 3 Nov 2026. */
export const TODAY = new Date(2026, 10, 3, 16, 30, 0);

function dayStart(offset: number): Date {
	const d = new Date(TODAY);
	d.setDate(d.getDate() - offset);
	return d;
}

function at(d: Date, hour: number, minute: number): string {
	const x = new Date(d);
	x.setHours(hour, minute, 0, 0);
	return x.toISOString();
}

export const METODE: Metode[] = [
	{
		id: 'ummi',
		nama: 'Ummi',
		template: true,
		level: [
			{ id: 'ummi-1', nama: 'Jilid 1 — Huruf Tunggal', jumlahHalaman: 40 },
			{ id: 'ummi-2', nama: 'Jilid 2 — Makhraj', jumlahHalaman: 40 },
			{ id: 'ummi-3', nama: 'Jilid 3 — Tanwin & Mad', jumlahHalaman: 44 },
			{ id: 'ummi-4', nama: 'Jilid 4 — Alif Lam', jumlahHalaman: 44 },
			{ id: 'ummi-5', nama: 'Jilid 5 — Idgam', jumlahHalaman: 48 },
			{ id: 'ummi-6', nama: 'Jilid 6 — Waqaf', jumlahHalaman: 48 }
		]
	},
	{
		id: 'wafa',
		nama: 'Wafa',
		template: true,
		level: [
			{ id: 'wafa-1', nama: 'Wafa 1', jumlahHalaman: 36 },
			{ id: 'wafa-2', nama: 'Wafa 2', jumlahHalaman: 36 },
			{ id: 'wafa-3', nama: 'Wafa 3', jumlahHalaman: 40 },
			{ id: 'wafa-4', nama: 'Wafa 4', jumlahHalaman: 40 },
			{ id: 'wafa-5', nama: 'Wafa 5', jumlahHalaman: 44 }
		]
	},
	{
		id: 'qiroati',
		nama: 'Qiroati',
		template: true,
		level: [
			{ id: 'qir-1', nama: 'Jilid 1', jumlahHalaman: 32 },
			{ id: 'qir-2', nama: 'Jilid 2', jumlahHalaman: 36 },
			{ id: 'qir-3', nama: 'Jilid 3', jumlahHalaman: 40 },
			{ id: 'qir-4', nama: 'Jilid 4', jumlahHalaman: 40 },
			{ id: 'qir-5', nama: 'Jilid 5', jumlahHalaman: 44 },
			{ id: 'qir-ghorib', nama: 'Ghorib', jumlahHalaman: 32 }
		]
	},
	{
		id: 'talaqqi',
		nama: 'Tilawati',
		template: true,
		level: [
			{ id: 'tal-1', nama: 'Tahsin Lanjutan', jumlahHalaman: 30 },
			{ id: 'tal-2', nama: 'Tahsin Tasmi', jumlahHalaman: 30 }
		]
	}
];

export const KELOMPOK: Kelompok[] = [
	{ id: 'k1', nama: 'Halaqah Ashar', jadwal: 'Sen, Rab, Jum · 16.00', metodeId: 'ummi', arsip: false },
	{ id: 'k2', nama: 'Halaqah Pagi', jadwal: 'Setiap hari · 06.30', metodeId: 'talaqqi', arsip: false },
	{ id: 'k3', nama: 'Kelas 4A Tahfidz', jadwal: 'Sel, Kam · 09.00', metodeId: 'qiroati', arsip: false }
];

const NAMA: [string, string][] = [
	['Ahmad Fauzan', 'k1'],
	['Muhammad Rizki', 'k1'],
	['Aisyah Nabila', 'k1'],
	['Hafidz Alfarizi', 'k1'],
	['Nurul Huda', 'k1'],
	['Zaki Ibrahim', 'k1'],
	['Salma Kamila', 'k1'],
	['Umar Abdurrahman', 'k1'],
	['Fathimah Azzahra', 'k1'],
	['Rayhan Maulana', 'k1'],
	['Khalid Fadhlurrahman', 'k1'],
	['Dzakiy Ramadhan', 'k1'],
	['Abdullah Hakim', 'k2'],
	['Yusuf Almutawakkil', 'k2'],
	['Maryam Hafizhah', 'k2'],
	['Ali Musthafa', 'k2'],
	['Zahra Shafiyyah', 'k2'],
	['Bilal Arkan', 'k2'],
	['Hasan Basri', 'k2'],
	['Umar Faruq', 'k2'],
	['Naila Putri', 'k3'],
	['Syifa Aulia', 'k3'],
	['Hanif Dhiya', 'k3'],
	['Arkan Naufal', 'k3'],
	['Farhan Aziz', 'k3'],
	['Alifah Zahrah', 'k3'],
	['Rafif Syarif', 'k3'],
	['Adiba Kalila', 'k3']
];

const TAG: Record<Jenis, string[]> = {
	ziyadah: ['Makhraj', 'Mad', 'Lupa', 'Tajwid', 'Waqaf'],
	murojaah: ['Makhraj', 'Lupa', 'Tajwid', 'Terputus'],
	tahsin: ['Makhraj', 'Mad', 'Idgam', 'Waqaf', 'Tajwid']
};

function makeSantri(): Santri[] {
	const rand = mulberry32(7);
	return NAMA.map(([nama, kelompokId], i) => {
		const arah: Arah = i % 7 === 0 ? 'mushaf' : 'juz30';
		const metodeId = kelompokId === 'k1' ? 'ummi' : kelompokId === 'k2' ? 'talaqqi' : 'qiroati';
		// Sebagian santri sudah punya bekal hafalan sebelum aplikasi dipakai.
		const jumlahAwal = 3 + Math.floor(rand() * 12);
		// Bekal harus searah dengan arah hafalan: urut mushaf mulai dari Al-Fatihah.
		const hafalAwal = arah === 'mushaf' ? [1] : JUZ30_REVERSE.slice(0, jumlahAwal).map((s) => s.n);
		return {
			id: `s${i + 1}`,
			nama,
			kelompokId,
			metodeId,
			arah,
			arsip: i >= NAMA.length - 2,
			hafalAwal,
			targetZiyadahPekan: rand() > 0.5 ? 3 : undefined
		};
	});
}

/** Kursor hafalan per santri, dipakai generator untuk membuat riwayat. */
interface Kursor {
	surat: number;
	ayat: number;
	levelIdx: number;
	halaman: number;
	urutan: number[]; // antrian surat sesuai arah hafalan
}

function queue(arah: Arah): number[] {
	return arah === 'juz30' ? JUZ30_REVERSE.map((s) => s.n) : SURAHS.map((s) => s.n);
}

function makeSetoran(santri: Santri[]): Setoran[] {
	const rand = mulberry32(20261103);
	const out: Setoran[] = [];
	const kursor = new Map<string, Kursor>();

	for (const s of santri) {
		const urutan = queue(s.arah);
		// Mulai setelah surat terakhir yang sudah dihafal.
		const last = s.hafalAwal.length ? urutan.indexOf(s.hafalAwal[s.hafalAwal.length - 1]) : -1;
		const mulaiIdx = last >= 0 ? last + 1 : s.arah === 'juz30' ? 10 : 0;
		kursor.set(s.id, {
			surat: urutan[Math.min(mulaiIdx, urutan.length - 1)],
			ayat: 1,
			levelIdx: 0,
			halaman: 1,
			urutan
		});
	}

	let seq = 0;
	// 42 hari riwayat. Hari mengajar tiap kelompok berbeda.
	for (let offset = 42; offset >= 0; offset--) {
		const day = dayStart(offset);
		const dow = day.getDay(); // 0 Minggu
		for (const s of santri) {
			if (s.arsip && offset < 20) continue;
			const aktif =
				s.kelompokId === 'k2' ||
				(s.kelompokId === 'k1' && [1, 3, 5].includes(dow)) ||
				(s.kelompokId === 'k3' && [2, 4].includes(dow));
			if (!aktif) continue;
			// tidak semua santri setor setiap hari
			if (rand() < (s.kelompokId === 'k2' ? 0.25 : 0.18)) continue;

			const k = kursor.get(s.id)!;
			const jenis: Jenis = rand() < 0.62 ? 'ziyadah' : rand() < 0.55 ? 'murojaah' : 'tahsin';
			const nilai = 2 + Math.floor(rand() * 4); // 2..5
			const record: Setoran = {
				id: `set-${++seq}`,
				santriId: s.id,
				kelompokSaatItu: s.kelompokId,
				jenis,
				nilai,
				tag: rand() < 0.3 ? [TAG[jenis][Math.floor(rand() * TAG[jenis].length)]] : [],
				dicatatPada: at(day, s.kelompokId === 'k2' ? 6 : s.kelompokId === 'k3' ? 9 : 16, 15 + Math.floor(rand() * 40))
			};

			if (jenis === 'ziyadah') {
				const info = surah(k.surat);
				const from = k.ayat;
				const sisa = info.ayat - from + 1;
				const span = Math.min(sisa, 1 + Math.floor(rand() * 8));
				const to = from + span - 1;
				record.suratAwal = k.surat;
				record.ayatAwal = from;
				record.suratAkhir = k.surat;
				record.ayatAkhir = to;
				if (nilai >= 3) {
					if (to >= info.ayat) {
						const idx = k.urutan.indexOf(k.surat);
						k.surat = k.urutan[Math.min(idx + 1, k.urutan.length - 1)];
						k.ayat = 1;
					} else {
						k.ayat = to + 1;
					}
				}
				if (rand() < 0.12)
					record.catatan = ['Ulang besok', 'Sudah lancar, lanjut', 'Perlu murajaah'][Math.floor(rand() * 3)];
			} else if (jenis === 'murojaah') {
				const pool = [...s.hafalAwal, ...k.urutan.slice(0, Math.max(0, k.urutan.indexOf(k.surat)))];
				const target = pool.length ? pool[Math.floor(rand() * pool.length)] : 112;
				const info = surah(target);
				record.suratAwal = target;
				record.ayatAwal = 1;
				record.suratAkhir = target;
				record.ayatAkhir = info.ayat;
			} else {
				const metode = METODE.find((m) => m.id === s.metodeId)!;
				const suratTahsin = k.urutan[seq % 10];
				record.suratAwal = suratTahsin;
				record.ayatAwal = 1;
				record.suratAkhir = suratTahsin;
				record.ayatAkhir = Math.min(surah(suratTahsin).ayat, 5);
				record.levelId = metode.level[0].id;
				record.halamanAwal = k.halaman;
				record.halamanAkhir = Math.min(k.halaman + 1, metode.level[0].jumlahHalaman);
				if (nilai >= 3) k.halaman = Math.min(k.halaman + 2, metode.level[0].jumlahHalaman);
			}
			out.push(record);
		}
	}

	// Hari ini: sebagian sudah setor supaya Beranda dan Mode Halaqah tidak kosong.
	const hariIni = santri.filter((s) => !s.arsip).slice(0, 14);
	hariIni.forEach((s, i) => {
		if (i % 3 === 2) return;
		const k = kursor.get(s.id)!;
		const info = surah(k.surat);
		const to = Math.min(k.ayat + 2, info.ayat);
		out.push({
			id: `set-${++seq}`,
			santriId: s.id,
			kelompokSaatItu: s.kelompokId,
			jenis: 'ziyadah',
			suratAwal: k.surat,
			ayatAwal: k.ayat,
			suratAkhir: k.surat,
			ayatAkhir: to,
			nilai: 3 + Math.floor(rand() * 3),
			tag: [],
			dicatatPada: at(TODAY, 16, 5 + i * 2)
		});
		if (to >= info.ayat) {
			const idx = k.urutan.indexOf(k.surat);
			k.surat = k.urutan[Math.min(idx + 1, k.urutan.length - 1)];
			k.ayat = 1;
		} else k.ayat = to + 1;
	});

	return out.sort((a, b) => a.dicatatPada.localeCompare(b.dicatatPada));
}

export const SANTRI: Santri[] = makeSantri();
export const SETORAN: Setoran[] = makeSetoran(SANTRI);

export const TARGET: Target[] = [
	{
		id: 't1',
		kelompokId: 'k1',
		jenis: 'ziyadah-pekan',
		nilaiTarget: 2,
		satuan: 'setoran/pekan',
		mulai: at(dayStart(30), 0, 0),
		tenggat: at(dayStart(-60), 0, 0)
	},
	{
		id: 't2',
		kelompokId: 'k2',
		jenis: 'hafalan',
		nilaiTarget: 0.5,
		satuan: 'juz',
		mulai: at(dayStart(60), 0, 0),
		tenggat: at(dayStart(-40), 0, 0)
	},
	{
		id: 't3',
		kelompokId: 'k3',
		jenis: 'tahsin',
		nilaiTarget: 1,
		satuan: 'level',
		mulai: at(dayStart(45), 0, 0),
		tenggat: at(dayStart(-135), 0, 0)
	},
	{
		id: 't4',
		santriId: 's4',
		jenis: 'ziyadah-pekan',
		nilaiTarget: 5,
		satuan: 'setoran/pekan',
		mulai: at(dayStart(20), 0, 0),
		tenggat: at(dayStart(-30), 0, 0)
	}
];

export const PENGAJAR: Pengajar = {
	nama: 'Ustadz Fulan',
	lembaga: 'Rumah Tahfidz Al-Fatih',
	email: 'fulan@contoh.id',
	plan: 'gratis',
	skala: 'huruf',
	metodeId: 'ummi',
	arah: 'juz30',
	ambangLulus: 3,
	tema: 'terang',
	batasHari: '00:00'
};

export { dayStart, at, TAG };
