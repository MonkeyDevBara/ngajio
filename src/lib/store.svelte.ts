import { JUZ30_REVERSE, SURAHS, surah } from './quran';
import { KELOMPOK, METODE, PENGAJAR, SANTRI, SETORAN, TARGET, TODAY } from './mock';
import { daysAgo, hariKey } from './format';
import * as sinkron from './sinkron';
import { api } from './sinkron';
import type { Data, Host, Perubahan, PenggunaAuth } from './sinkron';
import type {
	Arah,
	Jenis,
	Kelompok,
	Metode,
	Pengajar,
	Santri,
	Setoran,
	SkalaJenis,
	StatusSurat,
	Target
} from './types';

const KUNCI = 'ngajio:v1';
/** Penanda layar sambutan sudah dilihat. Terpisah agar tetap ada setelah keluar. */
const KUNCI_INTRO = 'ngajio:intro';
/** Keadaan mesin sinkron (lihat sinkron.ts). Dihapus saat keluar. */
const KUNCI_SINKRON = 'ngajio:sync:v1';
export const BATAS_GRATIS = 50;
/** Batas pergantian hari tetap tengah malam (Asia/Jakarta); tidak lagi bisa diatur. */
const BATAS_HARI = '00:00';
const AYAT_PER_JUZ = 6236 / 30;
const JENDELA_EDIT_HARI = 7;

/**
 * Jam "sekarang" versi demo: mengikuti jam asli, tetapi digeser agar jatuh pada
 * TODAY (3 Nov 2026, ba'da Ashar) saat dimuat. Riwayat contoh jadi selalu relevan.
 */
const OFFSET = TODAY.getTime() - Date.now();
export const sekarang = () => new Date(Date.now() + OFFSET);

const clone = <T>(x: T): T => structuredClone(x);
const uid = () =>
	typeof crypto !== 'undefined' && 'randomUUID' in crypto
		? crypto.randomUUID()
		: Math.random().toString(36).slice(2) + Date.now().toString(36);

export function antrianSurat(arah: Arah): number[] {
	return arah === 'juz30' ? JUZ30_REVERSE.map((s) => s.n) : SURAHS.map((s) => s.n);
}

export interface PosisiZiyadah {
	surat: number;
	ayat: number;
}
export interface HasilKemajuan {
	nilai: number;
	target: number;
	persen: number;
	label: string;
}
export interface Rekap {
	jumlah: number;
	perJenis: Record<Jenis, number>;
	rata: number | null;
	santriSetor: number;
	santriTotal: number;
	ayatBaru: number;
	ayatTahsin: number;
	hariAktif: number;
	perSantri: {
		santri: Santri;
		jumlah: number;
		ayat: number;
		ayatTahsin: number;
		rata: number | null;
		terakhir: string | null;
	}[];
}
export interface ToastData {
	id: number;
	pesan: string;
	aksi?: { label: string; fn: () => void };
}

interface Simpanan {
	v: 1;
	/** Aditif: tanpa mode berarti data lama (lihat muat). */
	mode?: 'contoh' | 'akun';
	akunSub?: string;
	sesi: boolean;
	onboarded: boolean;
	pengajar: Pengajar;
	metode: Metode[];
	kelompok: Kelompok[];
	santri: Santri[];
	setoran: Setoran[];
	target: Target[];
	ekspor: { terakhir: string | null; jumlah: number };
}

class Toko {
	siap = $state(false);
	sesi = $state(false);
	onboarded = $state(false);
	introSelesai = $state(false);

	pengajar = $state<Pengajar>(clone(PENGAJAR));
	metode = $state<Metode[]>(clone(METODE));
	kelompok = $state<Kelompok[]>(clone(KELOMPOK));
	santri = $state<Santri[]>(clone(SANTRI));
	setoran = $state<Setoran[]>(clone(SETORAN));
	target = $state<Target[]>(clone(TARGET));
	ekspor = $state<{ terakhir: string | null; jumlah: number }>({
		terakhir: null,
		jumlah: SETORAN.length
	});

	/** 'contoh': data lokal saja, tidak pernah disinkron. 'akun': tersinkron ke server. */
	mode = $state<'contoh' | 'akun'>('contoh');
	akunSub = $state<string | undefined>(undefined);
	/** Sekali pakai (tidak disimpan): tawarkan 'pasang di layar utama' setelah masuk. */
	tawarkanPasang = $state(false);

	/* Sinkronisasi: diisi mesin di sinkron.ts lewat `host`. */
	online = $state(true);
	antrian = $state(0);
	gagal = $state(false);
	sibuk = $state(false);
	sesiBerakhir = $state(false);
	sinkronTerakhir = $state<string | null>(null);

	toast = $state<ToastData | null>(null);
	private toastSeq = 0;
	private toastTimer: ReturnType<typeof setTimeout> | undefined;

	private host: Host = {
		potret: () => this.potret(),
		terapkan: (c) => this.terapkanDariServer(c),
		status: (s) => {
			this.antrian = s.antrian;
			this.sibuk = s.sibuk;
			this.gagal = s.gagal;
			this.sesiBerakhir = s.sesiBerakhir;
			this.sinkronTerakhir = s.tglSinkron;
		},
		toast: (p) => this.tampilToast(p)
	};

	/* ---------- Turunan ---------- */

	setoranAktif = $derived(this.setoran.filter((z) => !z.hapusPada));
	santriAktif = $derived(this.santri.filter((s) => !s.arsip));
	kelompokAktif = $derived(this.kelompok.filter((k) => !k.arsip));

	private perSantri = $derived.by(() => {
		const m = new Map<string, Setoran[]>();
		for (const z of this.setoranAktif) {
			const l = m.get(z.santriId);
			if (l) l.push(z);
			else m.set(z.santriId, [z]);
		}
		for (const l of m.values()) l.sort((a, b) => b.dicatatPada.localeCompare(a.dicatatPada));
		return m;
	});

	get efektifOnline() {
		return this.online;
	}
	get statusSinkron(): 'tersinkron' | 'menunggu' | 'gagal' {
		if (this.mode === 'contoh') return 'tersinkron';
		if (this.gagal) return 'gagal';
		if (!this.efektifOnline || this.antrian > 0 || this.sibuk) return 'menunggu';
		return 'tersinkron';
	}
	get gratis() {
		return this.pengajar.plan === 'gratis';
	}
	get sisaSlotSantri() {
		return this.gratis ? Math.max(0, BATAS_GRATIS - this.santriAktif.length) : Infinity;
	}
	get perluCadangan() {
		return this.setoranAktif.length - this.ekspor.jumlah > 50;
	}
	get batasMenit() {
		const [h, m] = BATAS_HARI.split(':').map(Number);
		return (h || 0) * 60 + (m || 0);
	}

	/* ---------- Siklus hidup & penyimpanan ---------- */

	muat() {
		if (this.siap || typeof localStorage === 'undefined') return;
		try {
			const raw = localStorage.getItem(KUNCI);
			if (raw) {
				const d = JSON.parse(raw) as Simpanan;
				if (d.v === 1) {
					// Data lama tanpa mode: contoh bila email-nya email mock, selain itu akun.
					this.mode = d.mode ?? (d.pengajar?.email === PENGAJAR.email ? 'contoh' : 'akun');
					this.akunSub = d.akunSub;
					this.sesi = d.sesi;
					this.onboarded = d.onboarded;
					this.pengajar = { ...clone(PENGAJAR), ...d.pengajar };
					// Data lama memakai default 'otomatis'; default sekarang terang.
					if (this.pengajar.tema === 'otomatis') this.pengajar.tema = 'terang';
					this.metode = d.metode;
					this.namakanTilawati();
					this.kelompok = d.kelompok;
					this.santri = d.santri;
					this.setoran = d.setoran;
					this.target = d.target;
					this.ekspor = d.ekspor;
				}
			}
		} catch {
			/* data rusak: pakai data contoh */
		}
		try {
			this.introSelesai = localStorage.getItem(KUNCI_INTRO) === '1';
		} catch {
			/* penyimpanan tidak tersedia */
		}
		this.online = typeof navigator === 'undefined' ? true : navigator.onLine;
		this.siap = true;
	}

	private potret() {
		return {
			pengajar: $state.snapshot(this.pengajar),
			metode: $state.snapshot(this.metode),
			kelompok: $state.snapshot(this.kelompok),
			santri: $state.snapshot(this.santri),
			setoran: $state.snapshot(this.setoran),
			target: $state.snapshot(this.target)
		};
	}

	private simpan(data: Data = this.potret()) {
		if (typeof localStorage === 'undefined') return;
		const d: Simpanan = {
			v: 1,
			mode: this.mode,
			akunSub: this.akunSub,
			sesi: this.sesi,
			onboarded: this.onboarded,
			...data,
			ekspor: $state.snapshot(this.ekspor)
		} as Simpanan;
		try {
			localStorage.setItem(KUNCI, JSON.stringify(d));
		} catch {
			/* penyimpanan penuh */
		}
	}

	/** Dipanggil setiap perubahan: simpan lokal, antre untuk sinkron. */
	private ubah() {
		const d = this.potret();
		this.simpan(d);
		if (this.mode === 'akun') sinkron.ubahan(d);
	}

	/** Metode bawaan "Talaqqi" berganti nama jadi "Tilawati". Id tetap supaya referensi santri dan setoran utuh. */
	private namakanTilawati() {
		let berubah = false;
		this.metode = this.metode.map((m) => {
			if (m.id !== 'talaqqi' || m.nama !== 'Talaqqi') return m;
			berubah = true;
			return { ...m, nama: 'Tilawati' };
		});
		return berubah;
	}

	/** Dipanggil saat aplikasi dibuka (layout) dan setelah masuk. Contoh tidak pernah sinkron. */
	mulaiSinkron() {
		if (this.mode === 'akun' && this.sesi && this.akunSub) sinkron.mulai(this.host, this.akunSub);
	}

	cobaLagiSinkron() {
		sinkron.cobaLagi();
	}

	/** Dipanggil layout untuk event online dan offline. */
	setOnline(v: boolean) {
		this.online = v;
		if (v && this.mode === 'akun') sinkron.picu();
	}
	picuSinkron() {
		if (this.mode === 'akun') sinkron.picu();
	}

	/** Terapkan perubahan tarikan ke store tanpa memicu sinkron balik. */
	terapkanDariServer(changes: Perubahan[]) {
		const per = new Map<string, Perubahan[]>();
		for (const c of changes) {
			const l = per.get(c.t);
			if (l) l.push(c);
			else per.set(c.t, [c]);
		}
		for (const c of per.get('pengajar') ?? []) {
			// foto opsional: record server mengganti seluruhnya, jadi foto lokal ikut hilang bila tidak ada.
			if (!c.hapus && c.data)
				this.pengajar = { ...this.pengajar, foto: undefined, ...(c.data as Partial<Pengajar>) };
		}
		const gabung = <T extends { id: string }>(arr: T[], cs: Perubahan[] | undefined): T[] => {
			if (!cs) return arr;
			const out = [...arr];
			const idx = new Map(out.map((r, i) => [r.id, i]));
			const buang = new Set<string>();
			for (const c of cs) {
				if (c.hapus) {
					buang.add(c.id);
					continue;
				}
				buang.delete(c.id);
				const r = { ...c.data, id: c.id } as unknown as T;
				const i = idx.get(c.id);
				if (i == null) {
					idx.set(c.id, out.length);
					out.push(r);
				} else out[i] = r;
			}
			return buang.size ? out.filter((r) => !buang.has(r.id)) : out;
		};
		this.metode = gabung(this.metode, per.get('metode'));
		this.namakanTilawati();
		this.kelompok = gabung(this.kelompok, per.get('kelompok'));
		this.santri = gabung(this.santri, per.get('santri'));
		this.setoran = gabung(this.setoran, per.get('setoran'));
		this.target = gabung(this.target, per.get('target'));
		this.simpan();
	}

	tampilToast(pesan: string, aksi?: { label: string; fn: () => void }) {
		const id = ++this.toastSeq;
		this.toast = { id, pesan, aksi };
		clearTimeout(this.toastTimer);
		this.toastTimer = setTimeout(() => {
			if (this.toast?.id === id) this.toast = null;
		}, 5000);
	}
	tutupToast() {
		this.toast = null;
	}

	/* ---------- Akun & data awal ---------- */

	private isiContoh() {
		this.pengajar = clone(PENGAJAR);
		this.metode = clone(METODE);
		this.kelompok = clone(KELOMPOK);
		this.santri = clone(SANTRI);
		this.setoran = clone(SETORAN);
		this.target = clone(TARGET);
		this.ekspor = { terakhir: null, jumlah: SETORAN.length };
		this.antrian = 0;
		this.gagal = false;
	}

	private isiKosong(email: string) {
		this.pengajar = {
			...clone(PENGAJAR),
			nama: '',
			lembaga: '',
			email,
			plan: 'gratis'
		};
		this.metode = clone(METODE);
		this.kelompok = [];
		this.santri = [];
		this.setoran = [];
		this.target = [];
		this.ekspor = { terakhir: null, jumlah: 0 };
		this.antrian = 0;
		this.gagal = false;
	}

	selesaikanIntro() {
		this.introSelesai = true;
		if (typeof localStorage !== 'undefined') localStorage.setItem(KUNCI_INTRO, '1');
	}

	/**
	 * Masuk dengan akun server (aturan login pertama, docs/API.md). Melempar bila tarikan
	 * gagal; saat itu data lokal tidak disentuh.
	 */
	async masukAkun(user: PenggunaAuth) {
		const sub = String(user.id);
		const sama = this.mode === 'akun' && this.sesi && this.akunSub === sub;
		if (!sama) {
			const akunLain = this.mode === 'akun' && !!this.akunSub && this.akunSub !== sub;
			if (akunLain && this.antrian > 0) {
				throw new Error(
					'Ada perubahan akun sebelumnya di HP ini yang belum terkirim. Masuk dengan akun itu lagi, atau keluar lewat Pengaturan.'
				);
			}
			// Data contoh tidak pernah diunggah. Data akun lama (tanpa akunSub) diunggah bila server kosong.
			const lokalNyata =
				this.mode === 'akun' && !akunLain && (this.santri.length > 0 || this.setoran.length > 0);
			const tarik = await sinkron.tarikSemua();
			if (tarik.changes.length) {
				this.isiKosong(user.email);
				this.metode = [];
				this.terapkanDariServer(tarik.changes);
				if (!this.metode.length) this.metode = clone(METODE);
				this.onboarded = this.pengajar.nama !== '';
				this.ekspor = { terakhir: null, jumlah: this.setoranAktif.length };
				sinkron.inisialisasi(sub, tarik.cursor, tarik.changes);
			} else if (lokalNyata) {
				sinkron.inisialisasi(sub, tarik.cursor, []);
			} else {
				this.isiKosong(user.email);
				this.onboarded = false;
				sinkron.inisialisasi(sub, tarik.cursor, []);
			}
			this.mode = 'akun';
			this.akunSub = sub;
		}
		this.pengajar = { ...this.pengajar, email: user.email, plan: user.plan };
		this.sesi = true;
		this.tawarkanPasang = true;
		this.simpan();
		this.mulaiSinkron();
	}

	/** Masuk dengan data contoh (28 santri, 42 hari riwayat). Tidak pernah sinkron. */
	masukContoh() {
		sinkron.berhenti();
		this.isiContoh();
		this.mode = 'contoh';
		this.akunSub = undefined;
		this.sesi = true;
		this.onboarded = true;
		this.simpan();
	}

	muatUlangContoh() {
		this.masukContoh();
	}

	keluar() {
		if (this.mode === 'akun') api('/api/auth/logout', {}).catch(() => {});
		sinkron.hapusSimpanan();
		this.isiContoh();
		this.mode = 'contoh';
		this.akunSub = undefined;
		this.sesiBerakhir = false;
		this.sinkronTerakhir = null;
		this.sesi = false;
		this.onboarded = false;
		if (typeof localStorage !== 'undefined') {
			localStorage.removeItem(KUNCI);
			localStorage.removeItem(KUNCI_SINKRON);
		}
	}

	selesaikanSetup(p: {
		nama: string;
		lembaga: string;
		metodeId: string;
		skala: SkalaJenis;
		kelompok: string;
		namaSantri: string[];
	}) {
		this.pengajar = {
			...this.pengajar,
			nama: p.nama,
			lembaga: p.lembaga,
			skala: p.skala,
			metodeId: p.metodeId
		};
		const kid = uid();
		this.kelompok = [
			{ id: kid, nama: p.kelompok || 'Halaqah 1', jadwal: '', metodeId: p.metodeId, arsip: false }
		];
		this.santri = p.namaSantri.map((nama) => ({
			id: uid(),
			nama,
			kelompokId: kid,
			metodeId: p.metodeId,
			arah: this.pengajar.arah,
			arsip: false,
			hafalAwal: []
		}));
		this.onboarded = true;
		this.ubah();
		return kid;
	}

	ubahPengajar(patch: Partial<Pengajar>) {
		this.pengajar = { ...this.pengajar, ...patch };
		this.ubah();
	}

	/* ---------- Pencarian dasar ---------- */

	santriById(id: string | null | undefined) {
		return id ? this.santri.find((s) => s.id === id) : undefined;
	}
	kelompokById(id: string | null | undefined) {
		return id ? this.kelompok.find((k) => k.id === id) : undefined;
	}
	metodeById(id: string | null | undefined) {
		return this.metode.find((m) => m.id === id) ?? this.metode[0];
	}
	setoranSantri(id: string): Setoran[] {
		return this.perSantri.get(id) ?? [];
	}
	anggota(kelompokId: string, termasukArsip = false) {
		return this.santri.filter((s) => s.kelompokId === kelompokId && (termasukArsip || !s.arsip));
	}

	hariBisnis(d: Date | string): string {
		const t = typeof d === 'string' ? new Date(d) : d;
		return hariKey(new Date(t.getTime() - this.batasMenit * 60000));
	}
	hariIniKey() {
		return this.hariBisnis(sekarang());
	}
	setoranHariIni(santriId?: string): Setoran[] {
		const hi = this.hariIniKey();
		const src = santriId ? this.setoranSantri(santriId) : this.setoranAktif;
		return src.filter((z) => this.hariBisnis(z.dicatatPada) === hi);
	}

	/* ---------- Posisi otomatis (PRD bagian 8) ---------- */

	posisiZiyadah(santriId: string): PosisiZiyadah {
		const s = this.santriById(santriId);
		const urutan = antrianSurat(s?.arah ?? this.pengajar.arah);
		const ambang = this.pengajar.ambangLulus;
		const list = this.setoranSantri(santriId).filter(
			(z) => z.jenis === 'ziyadah' && z.suratAkhir != null
		);
		const lulus = list.find((z) => z.nilai >= ambang);
		if (lulus) {
			const info = surah(lulus.suratAkhir!);
			if ((lulus.ayatAkhir ?? 0) >= info.ayat) {
				const idx = urutan.indexOf(lulus.suratAkhir!);
				return { surat: urutan[Math.min(idx + 1, urutan.length - 1)], ayat: 1 };
			}
			return { surat: lulus.suratAkhir!, ayat: (lulus.ayatAkhir ?? 0) + 1 };
		}
		// Belum ada setoran lulus: ulangi bagian yang pernah dicoba.
		const pertama = list[list.length - 1];
		if (pertama?.suratAwal != null) return { surat: pertama.suratAwal, ayat: pertama.ayatAwal ?? 1 };
		const hafal = s?.hafalAwal ?? [];
		const last = hafal.length ? urutan.indexOf(hafal[hafal.length - 1]) : -1;
		return { surat: urutan[Math.min(last + 1, urutan.length - 1)], ayat: 1 };
	}

	/** Jumlah ayat default: mengikuti panjang setoran lulus terakhir. */
	spanDefault(santriId: string): number {
		const z = this.setoranSantri(santriId).find(
			(x) => x.jenis === 'ziyadah' && x.nilai >= this.pengajar.ambangLulus && x.ayatAwal != null
		);
		if (!z || z.ayatAkhir == null || z.ayatAwal == null) return 3;
		return Math.max(1, z.ayatAkhir - z.ayatAwal + 1);
	}

	/** Level terakhir yang diisi pada setoran tahsin santri (kosong bila belum pernah). */
	levelTahsinTerakhir(santriId: string): string {
		return this.setoranSantri(santriId).find((z) => z.jenis === 'tahsin' && z.levelId)?.levelId ?? '';
	}

	levelById(id: string | undefined) {
		return this.metode.flatMap((m) => m.level).find((l) => l.id === id);
	}

	/* ---------- Status surat & murojaah ---------- */

	statusSurat(santriId: string, sampai?: string): Map<number, StatusSurat> {
		const out = new Map<number, StatusSurat>();
		const s = this.santriById(santriId);
		if (!s) return out;
		for (const n of s.hafalAwal) out.set(n, 'hafal');
		const ambang = this.pengajar.ambangLulus;
		const cover = new Map<number, Set<number>>();
		for (const z of this.setoranSantri(santriId)) {
			if (z.jenis !== 'ziyadah' || z.suratAwal == null || z.ayatAwal == null) continue;
			if (sampai && z.dicatatPada > sampai) continue;
			if (z.nilai < ambang) {
				if (!out.has(z.suratAwal)) out.set(z.suratAwal, 'sedang');
				continue;
			}
			const set = cover.get(z.suratAwal) ?? new Set<number>();
			for (let a = z.ayatAwal; a <= (z.ayatAkhir ?? z.ayatAwal); a++) set.add(a);
			cover.set(z.suratAwal, set);
		}
		for (const [n, set] of cover) {
			if (out.get(n) === 'hafal') continue;
			out.set(n, set.size >= surah(n).ayat ? 'hafal' : 'sedang');
		}
		return out;
	}

	ayatHafal(santriId: string, sampai?: string): number {
		let total = 0;
		for (const [n, st] of this.statusSurat(santriId, sampai)) if (st === 'hafal') total += surah(n).ayat;
		return total;
	}

	/** Ayat yang sudah lulus ziyadah, termasuk surat yang belum tuntas. */
	ayatLulus(santriId: string, sampai?: string): number {
		const s = this.santriById(santriId);
		if (!s) return 0;
		const ambang = this.pengajar.ambangLulus;
		const awal = new Set(s.hafalAwal);
		let total = 0;
		for (const n of awal) total += surah(n).ayat;
		const cover = new Map<number, Set<number>>();
		for (const z of this.setoranSantri(santriId)) {
			if (z.jenis !== 'ziyadah' || z.nilai < ambang || z.suratAwal == null || z.ayatAwal == null) continue;
			if (sampai && z.dicatatPada > sampai) continue;
			if (awal.has(z.suratAwal)) continue;
			const set = cover.get(z.suratAwal) ?? new Set<number>();
			for (let a = z.ayatAwal; a <= (z.ayatAkhir ?? z.ayatAwal); a++) set.add(a);
			cover.set(z.suratAwal, set);
		}
		for (const set of cover.values()) total += set.size;
		return total;
	}

	jumlahHafal(santriId: string, sampai?: string): number {
		let c = 0;
		for (const st of this.statusSurat(santriId, sampai).values()) if (st === 'hafal') c++;
		return c;
	}

	murojaahTerakhir(santriId: string): Map<number, string> {
		const m = new Map<number, string>();
		for (const z of this.setoranSantri(santriId)) {
			if (z.jenis === 'murojaah' && z.suratAwal != null && !m.has(z.suratAwal)) {
				m.set(z.suratAwal, z.dicatatPada);
			}
		}
		return m;
	}

	/** F-19: maksimal 3 surat hafal dengan murojaah paling lama. */
	saranMurojaah(santriId: string, n = 3): { surat: number; hari: number | null }[] {
		const terakhir = this.murojaahTerakhir(santriId);
		const hi = this.hariIniKey();
		const out: { surat: number; hari: number | null; iso: string }[] = [];
		for (const [surat, st] of this.statusSurat(santriId)) {
			if (st !== 'hafal') continue;
			const iso = terakhir.get(surat);
			if (iso && this.hariBisnis(iso) === hi) continue;
			out.push({ surat, iso: iso ?? '', hari: iso ? daysAgo(iso, sekarang()) : null });
		}
		out.sort((a, b) => a.iso.localeCompare(b.iso) || b.surat - a.surat);
		return out.slice(0, n).map(({ surat, hari }) => ({ surat, hari }));
	}

	streak(santriId: string): number {
		const hari = new Set(this.setoranSantri(santriId).map((z) => this.hariBisnis(z.dicatatPada)));
		let d = sekarang();
		if (!hari.has(this.hariBisnis(d))) d = new Date(d.getTime() - 86400000);
		let n = 0;
		while (hari.has(this.hariBisnis(d))) {
			n++;
			d = new Date(d.getTime() - 86400000);
		}
		return n;
	}

	/* ---------- Target ---------- */

	/** Target kelompok berlaku ke semua anggota; target santri menimpa jenis yang sama. */
	targetEfektif(santriId: string): Target[] {
		const s = this.santriById(santriId);
		if (!s) return [];
		const now = sekarang().toISOString();
		const aktif = (t: Target) => t.tenggat >= now;
		const milik = this.target.filter((t) => t.santriId === santriId && aktif(t));
		const kelompok = this.target.filter(
			(t) => t.kelompokId === s.kelompokId && aktif(t) && !milik.some((m) => m.jenis === t.jenis)
		);
		return [...milik, ...kelompok];
	}

	/** Level tahsin tertinggi yang pernah lulus (indeks level 0-based, ditambah pecahan halaman bila diisi). */
	private kemajuanTahsin(santriId: string, sampai?: string): number {
		const s = this.santriById(santriId);
		const m = this.metodeById(s?.metodeId);
		const ambang = this.pengajar.ambangLulus;
		let terbaik = 0;
		for (const z of this.setoranSantri(santriId)) {
			if (z.jenis !== 'tahsin' || z.nilai < ambang || !z.levelId) continue;
			if (sampai && z.dicatatPada > sampai) continue;
			const idx = m.level.findIndex((l) => l.id === z.levelId);
			if (idx < 0) continue;
			terbaik = Math.max(terbaik, idx + (z.halamanAkhir ?? 0) / (m.level[idx].jumlahHalaman || 1));
		}
		return terbaik;
	}

	/** Progress dihitung dari setoran, tidak disimpan sebagai angka terpisah. */
	progressTarget(t: Target, santriId: string): HasilKemajuan {
		const target = t.nilaiTarget;
		let nilai = 0;
		let label = '';
		if (t.jenis === 'ziyadah-pekan') {
			const batas = new Date(sekarang().getTime() - 7 * 86400000).toISOString();
			nilai = this.setoranSantri(santriId).filter(
				(z) => z.jenis === 'ziyadah' && z.dicatatPada >= batas
			).length;
			label = `${nilai} dari ${target} setoran dalam 7 hari`;
		} else if (t.jenis === 'hafalan') {
			if (t.satuan === 'surat') {
				nilai = Math.max(0, this.jumlahHafal(santriId) - this.jumlahHafal(santriId, t.mulai));
				label = `${nilai} dari ${target} surat`;
			} else {
				const ayat = this.ayatLulus(santriId) - this.ayatLulus(santriId, t.mulai);
				nilai = Math.max(0, ayat) / AYAT_PER_JUZ;
				label = `${nilai.toLocaleString('id-ID', { maximumFractionDigits: 1 })} dari ${target.toLocaleString('id-ID')} juz`;
			}
		} else {
			nilai = Math.max(0, this.kemajuanTahsin(santriId) - this.kemajuanTahsin(santriId, t.mulai));
			label = `${nilai.toLocaleString('id-ID', { maximumFractionDigits: 1 })} dari ${target.toLocaleString('id-ID')} level`;
		}
		return { nilai, target, persen: target > 0 ? Math.min(1, nilai / target) : 0, label };
	}

	/** Rata-rata progress semua target santri (milik sendiri + dari kelompok). */
	ringkasTarget(santriId: string): { persen: number; jumlah: number } | null {
		const ts = this.targetEfektif(santriId);
		if (!ts.length) return null;
		const persen = ts.reduce((a, t) => a + this.progressTarget(t, santriId).persen, 0) / ts.length;
		return { persen, jumlah: ts.length };
	}

	/** F-17: tanpa ziyadah 7 hari, atau tertinggal dari target. */
	tertinggal(kelompokId?: string): { santri: Santri; alasan: string; hari: number | null }[] {
		const out: { santri: Santri; alasan: string; hari: number | null }[] = [];
		for (const s of this.santriAktif) {
			if (kelompokId && s.kelompokId !== kelompokId) continue;
			const z = this.setoranSantri(s.id).find((x) => x.jenis === 'ziyadah');
			const hari = z ? daysAgo(z.dicatatPada, sekarang()) : null;
			const alasan: string[] = [];
			if (hari === null) alasan.push('Belum pernah setor hafalan baru');
			else if (hari >= 7) alasan.push(`${hari} hari tanpa hafalan baru`);
			for (const t of this.targetEfektif(s.id)) {
				const p = this.progressTarget(t, s.id);
				const total = new Date(t.tenggat).getTime() - new Date(t.mulai).getTime();
				const lewat = sekarang().getTime() - new Date(t.mulai).getTime();
				// Target mingguan: tertinggal bila kurang dari separuh. Target lain: bandingkan dengan laju waktu.
				const tertinggalDariTarget =
					t.jenis === 'ziyadah-pekan'
						? p.persen < 0.5
						: p.persen + 0.15 < Math.min(1, Math.max(0, lewat / total));
				if (tertinggalDariTarget && !alasan.length) alasan.push(`Di bawah target: ${p.label}`);
			}
			if (alasan.length) out.push({ santri: s, alasan: alasan[0], hari });
		}
		return out.sort((a, b) => (b.hari ?? 999) - (a.hari ?? 999));
	}

	/** Kelompok yang jadwalnya jatuh pada hari ini (berdasarkan singkatan hari di jadwal). */
	kelompokHariIni(): Kelompok[] {
		const nama = ['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab'];
		const h = nama[sekarang().getDay()];
		const cocok = this.kelompokAktif.filter(
			(k) => k.jadwal.includes('Setiap hari') || k.jadwal.includes(h)
		);
		return cocok.length ? cocok : this.kelompokAktif;
	}

	/* ---------- Mode Halaqah ---------- */

	urutanHalaqah(kelompokId: string): Santri[] {
		const k = this.kelompokById(kelompokId);
		const ang = this.anggota(kelompokId);
		const u = k?.urutan ?? [];
		return [...ang].sort((a, b) => {
			const ia = u.indexOf(a.id);
			const ib = u.indexOf(b.id);
			return (ia < 0 ? 9999 : ia) - (ib < 0 ? 9999 : ib);
		});
	}

	geser(kelompokId: string, santriId: string, delta: -1 | 1) {
		const k = this.kelompokById(kelompokId);
		if (!k) return;
		const ids = this.urutanHalaqah(kelompokId).map((s) => s.id);
		const i = ids.indexOf(santriId);
		const j = i + delta;
		if (i < 0 || j < 0 || j >= ids.length) return;
		[ids[i], ids[j]] = [ids[j], ids[i]];
		k.urutan = ids;
		this.ubah();
	}

	/* ---------- Setoran ---------- */

	tambahSetoran(
		input: Omit<Setoran, 'id' | 'dicatatPada' | 'kelompokSaatItu' | 'tag'> & { tag?: string[] }
	): Setoran {
		const s = this.santriById(input.santriId);
		const z: Setoran = {
			...input,
			tag: input.tag ?? [],
			id: uid(),
			kelompokSaatItu: s?.kelompokId ?? '',
			dicatatPada: sekarang().toISOString()
		};
		this.setoran.push(z);
		this.ubah();
		return z;
	}

	bisaDiubah(z: Setoran): boolean {
		return daysAgo(z.dicatatPada, sekarang()) <= JENDELA_EDIT_HARI;
	}

	ubahSetoran(id: string, patch: Partial<Setoran>) {
		const z = this.setoran.find((x) => x.id === id);
		if (!z) return;
		Object.assign(z, patch, { diubahPada: sekarang().toISOString() });
		this.ubah();
	}

	/** Hapus = tombstone. Riwayat di laporan lama tetap konsisten. */
	hapusSetoran(id: string) {
		const z = this.setoran.find((x) => x.id === id);
		if (!z) return;
		z.hapusPada = sekarang().toISOString();
		this.ubah();
	}
	pulihkanSetoran(id: string) {
		const z = this.setoran.find((x) => x.id === id);
		if (!z) return;
		delete z.hapusPada;
		this.ubah();
	}

	/* ---------- Santri & kelompok ---------- */

	tambahSantri(
		daftar: { nama: string; kelompokId: string; waWali?: string }[]
	): { ok: boolean; pesan?: string; jumlah: number } {
		if (!this.efektifOnline) {
			return {
				ok: false,
				jumlah: 0,
				pesan:
					'Menambah santri butuh koneksi internet supaya batas paket bisa diperiksa. Setoran santri yang sudah ada tetap bisa dicatat.'
			};
		}
		if (this.gratis && this.santriAktif.length + daftar.length > BATAS_GRATIS) {
			const sisa = this.sisaSlotSantri;
			return {
				ok: false,
				jumlah: 0,
				pesan:
					sisa > 0
						? `Paket gratis maksimal ${BATAS_GRATIS} santri aktif. Anda baru bisa menambah ${sisa} santri lagi. Arsipkan santri yang sudah tidak ikut atau upgrade paket.`
						: `Paket gratis maksimal ${BATAS_GRATIS} santri aktif dan sudah penuh. Setoran santri yang ada tetap bisa dicatat. Arsipkan santri yang sudah tidak ikut atau upgrade paket.`
			};
		}
		for (const d of daftar) {
			const k = this.kelompokById(d.kelompokId);
			this.santri.push({
				id: uid(),
				nama: d.nama,
				kelompokId: d.kelompokId,
				metodeId: k?.metodeId ?? this.pengajar.metodeId,
				arah: this.pengajar.arah,
				waWali: d.waWali,
				arsip: false,
				hafalAwal: []
			});
		}
		this.ubah();
		return { ok: true, jumlah: daftar.length };
	}

	ubahSantri(id: string, patch: Partial<Santri>) {
		const s = this.santriById(id);
		if (!s) return;
		Object.assign(s, patch);
		this.ubah();
	}

	/** Riwayat mengikuti santri; setoran lama tetap membawa kelompok asalnya. */
	pindahKelompok(id: string, kelompokId: string) {
		const s = this.santriById(id);
		const k = this.kelompokById(kelompokId);
		if (!s || !k) return;
		s.kelompokId = kelompokId;
		this.ubah();
	}

	arsipkanSantri(id: string, arsip: boolean) {
		const s = this.santriById(id);
		if (!s) return false;
		if (!arsip && this.gratis && this.santriAktif.length >= BATAS_GRATIS) return false;
		s.arsip = arsip;
		this.ubah();
		return true;
	}

	tambahKelompok(p: { nama: string; jadwal: string; metodeId: string }) {
		const k: Kelompok = { id: uid(), arsip: false, ...p };
		this.kelompok.push(k);
		this.ubah();
		return k;
	}
	ubahKelompok(id: string, patch: Partial<Kelompok>) {
		const k = this.kelompokById(id);
		if (!k) return;
		Object.assign(k, patch);
		this.ubah();
	}

	tambahTarget(t: Omit<Target, 'id'>) {
		this.target.push({ ...t, id: uid() });
		this.ubah();
	}
	hapusTarget(id: string) {
		this.target = this.target.filter((t) => t.id !== id);
		this.ubah();
	}

	/* ---------- Metode tahsin ---------- */

	simpanMetode(m: Metode) {
		const i = this.metode.findIndex((x) => x.id === m.id);
		if (i >= 0) this.metode[i] = m;
		else this.metode.push(m);
		this.ubah();
	}
	hapusMetode(id: string) {
		const dipakai =
			this.santri.some((s) => s.metodeId === id) || this.kelompok.some((k) => k.metodeId === id);
		if (dipakai || this.metode.length <= 1) return false;
		this.metode = this.metode.filter((m) => m.id !== id);
		if (this.pengajar.metodeId === id) this.pengajar.metodeId = this.metode[0].id;
		this.ubah();
		return true;
	}
	buatId() {
		return uid();
	}

	/* ---------- Ekspor ---------- */

	catatEkspor() {
		this.ekspor = { terakhir: sekarang().toISOString(), jumlah: this.setoranAktif.length };
		this.simpan();
	}

	/* ---------- Laporan ---------- */

	rekap(opt: { kelompokId?: string; santriId?: string; dari: Date; sampai: Date }): Rekap {
		const dari = opt.dari.toISOString();
		const sampai = opt.sampai.toISOString();
		const ambang = this.pengajar.ambangLulus;
		const list = this.setoranAktif.filter((z) => {
			if (z.dicatatPada < dari || z.dicatatPada > sampai) return false;
			if (opt.santriId) return z.santriId === opt.santriId;
			if (opt.kelompokId) return z.kelompokSaatItu === opt.kelompokId;
			return true;
		});

		const perJenis: Record<Jenis, number> = { ziyadah: 0, murojaah: 0, tahsin: 0 };
		const hari = new Set<string>();
		const per = new Map<string, Setoran[]>();
		let jumlahNilai = 0;
		let ayat = 0;
		let ayatTh = 0;
		for (const z of list) {
			perJenis[z.jenis]++;
			jumlahNilai += z.nilai;
			hari.add(this.hariBisnis(z.dicatatPada));
			if (z.nilai >= ambang) {
				if (z.jenis === 'ziyadah') ayat += (z.ayatAkhir ?? 0) - (z.ayatAwal ?? 0) + 1;
				if (z.jenis === 'tahsin' && z.suratAwal != null) ayatTh += (z.ayatAkhir ?? 0) - (z.ayatAwal ?? 0) + 1;
			}
			const l = per.get(z.santriId);
			if (l) l.push(z);
			else per.set(z.santriId, [z]);
		}

		// Santri aktif di ruang lingkup ikut tampil walau belum setor. Santri arsip
		// tetap tampil bila punya setoran pada periode ini.
		const ids = new Set(per.keys());
		for (const s of this.santriAktif) {
			if (opt.santriId && s.id !== opt.santriId) continue;
			if (opt.kelompokId && s.kelompokId !== opt.kelompokId) continue;
			ids.add(s.id);
		}
		const perSantri = [...ids]
			.map((id) => {
				const santri = this.santriById(id)!;
				const zs = per.get(id) ?? [];
				const nz = zs.filter((z) => z.jenis === 'ziyadah' && z.nilai >= ambang);
				const nt = zs.filter((z) => z.jenis === 'tahsin' && z.nilai >= ambang);
				return {
					santri,
					jumlah: zs.length,
					ayat: nz.reduce((a, z) => a + (z.ayatAkhir ?? 0) - (z.ayatAwal ?? 0) + 1, 0),
					ayatTahsin: nt.reduce((a, z) => a + (z.suratAwal == null ? 0 : (z.ayatAkhir ?? 0) - (z.ayatAwal ?? 0) + 1), 0),
					rata: zs.length ? zs.reduce((a, z) => a + z.nilai, 0) / zs.length : null,
					terakhir: zs.length ? zs[0].dicatatPada : null
				};
			})
			.filter((r) => r.santri)
			.sort((a, b) => b.jumlah - a.jumlah || a.santri.nama.localeCompare(b.santri.nama));

		return {
			jumlah: list.length,
			perJenis,
			rata: list.length ? jumlahNilai / list.length : null,
			santriSetor: per.size,
			santriTotal: ids.size,
			ayatBaru: ayat,
			ayatTahsin: ayatTh,
			hariAktif: hari.size,
			perSantri
		};
	}
}

export const app = new Toko();
