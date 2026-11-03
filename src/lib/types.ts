export type Jenis = 'ziyadah' | 'murojaah' | 'tahsin';
export type Arah = 'juz30' | 'mushaf';
export type SkalaJenis = 'huruf' | 'bintang' | 'predikat' | 'angka';
export type JenisTarget = 'hafalan' | 'ziyadah-pekan' | 'tahsin';
export type Tema = 'otomatis' | 'terang' | 'gelap';
export type StatusSurat = 'hafal' | 'sedang' | 'kosong';

export interface MetodeLevel {
	id: string;
	nama: string;
	jumlahHalaman: number;
}

export interface Metode {
	id: string;
	nama: string;
	template: boolean;
	level: MetodeLevel[];
}

export interface Kelompok {
	id: string;
	nama: string;
	jadwal: string;
	metodeId: string;
	arsip: boolean;
	/** Urutan santri di Mode Halaqah (id santri). Yang tidak terdaftar ditaruh di akhir. */
	urutan?: string[];
}

export interface Santri {
	id: string;
	nama: string;
	kelompokId: string;
	metodeId: string;
	arah: Arah;
	waWali?: string;
	arsip: boolean;
	/** Surat yang sudah hafal sebelum riwayat setoran dimulai. */
	hafalAwal: number[];
	targetZiyadahPekan?: number;
}

export interface Setoran {
	id: string;
	santriId: string;
	kelompokSaatItu: string;
	jenis: Jenis;
	suratAwal?: number;
	ayatAwal?: number;
	suratAkhir?: number;
	ayatAkhir?: number;
	levelId?: string;
	halamanAwal?: number;
	halamanAkhir?: number;
	nilai: number;
	catatan?: string;
	tag: string[];
	dicatatPada: string;
	diubahPada?: string;
	/** Tombstone: setoran dihapus tetap disimpan supaya laporan lama konsisten. */
	hapusPada?: string;
}

export interface Target {
	id: string;
	santriId?: string;
	kelompokId?: string;
	jenis: JenisTarget;
	nilaiTarget: number;
	satuan: string;
	mulai: string;
	tenggat: string;
}

export interface Pengajar {
	nama: string;
	lembaga: string;
	email: string;
	plan: 'gratis' | 'bayar';
	skala: SkalaJenis;
	metodeId: string;
	arah: Arah;
	ambangLulus: number;
	tema: Tema;
	/** Jam batas pergantian hari, format HH:MM (zona Asia/Jakarta). */
	batasHari: string;
	/** Foto profil kecil: data URL JPEG (lihat foto.ts). */
	foto?: string;
}
