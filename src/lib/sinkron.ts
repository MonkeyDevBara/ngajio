/**
 * Mesin sinkron (docs/API.md). Tidak mengimpor store: store memasang `Host`
 * lewat `mulai()`, jadi tidak ada impor melingkar.
 * Semua pewaktuan di sini memakai Date.now() asli, bukan sekarang().
 */

export type Tabel = 'pengajar' | 'metode' | 'kelompok' | 'santri' | 'setoran' | 'target';
export interface Perubahan {
	t: Tabel;
	id: string;
	rev?: number;
	data?: Record<string, unknown>;
	hapus?: boolean;
}
interface Entri {
	mid: string;
	t: Tabel;
	id: string;
	data?: Record<string, unknown>;
	hapus?: true;
}
interface Simpanan {
	v: 1;
	sub: string;
	cursor: number;
	shadow: Record<string, string>;
	outbox: Entri[];
	tglSinkron: string | null;
}
interface BalasanSinkron {
	cursor: number;
	changes: Perubahan[];
	more: boolean;
	rejected?: { mid: string; code: string; message: string }[];
}
/** Potret data store (objek biasa, bukan proxy $state). */
export interface Data {
	pengajar: object;
	metode: object[];
	kelompok: object[];
	santri: object[];
	setoran: object[];
	target: object[];
}
export interface Host {
	potret(): Data;
	terapkan(changes: Perubahan[]): void;
	status(s: { antrian: number; sibuk: boolean; gagal: boolean; sesiBerakhir: boolean; tglSinkron: string | null }): void;
	toast(pesan: string): void;
}
export interface PenggunaAuth {
	id: number;
	email: string;
	nama: string;
	plan: 'gratis' | 'bayar';
}
export interface BalasanAuth {
	user: PenggunaAuth;
	baru: boolean;
	cursor: number;
}

/* ---------- Klien HTTP ---------- */

export class ApiError extends Error {
	constructor(
		public status: number,
		public code: string,
		message: string
	) {
		super(message);
	}
}

export async function api<T>(path: string, body?: unknown, metode: 'GET' | 'POST' = 'POST'): Promise<T> {
	const res = await fetch(path, {
		method: metode,
		headers: { 'Content-Type': 'application/json', 'X-Ngajio': '1' },
		credentials: 'same-origin',
		body: metode === 'POST' ? JSON.stringify(body ?? {}) : undefined
	});
	let j: { error?: { code?: string; message?: string } } | null = null;
	try {
		j = await res.json();
	} catch {
		/* badan kosong atau bukan JSON */
	}
	if (!res.ok) throw new ApiError(res.status, j?.error?.code ?? 'error', j?.error?.message ?? `HTTP ${res.status}`);
	return j as T;
}

/** Pesan galat untuk pengguna (Bahasa Indonesia). */
export function pesanGalat(e: unknown): string {
	if (e instanceof ApiError) {
		if (e.status === 401) return 'Masuk gagal. Coba lagi.';
		if (e.status === 429) return 'Terlalu banyak percobaan. Tunggu sebentar lalu coba lagi.';
		if (e.status === 503) return 'Masuk Google belum dikonfigurasi di server.';
		return `Server menolak permintaan (${e.status}).`;
	}
	return e instanceof Error && e.message && !(e instanceof TypeError)
		? e.message
		: 'Tidak bisa terhubung ke server. Periksa sinyal Anda.';
}

/* ---------- Hash ---------- */

const kanon = (v: unknown): string => {
	if (Array.isArray(v)) return '[' + v.map(kanon).join(',') + ']';
	if (v && typeof v === 'object') {
		const o = v as Record<string, unknown>;
		return (
			'{' +
			Object.keys(o)
				.filter((k) => o[k] !== undefined)
				.sort()
				.map((k) => JSON.stringify(k) + ':' + kanon(o[k]))
				.join(',') +
			'}'
		);
	}
	return JSON.stringify(v) ?? 'null';
};

/** FNV-1a 64-bit atas JSON kanonis, dihitung dengan dua separuh 32-bit (tanpa BigInt). */
function hash(v: unknown): string {
	const s = kanon(v);
	let hi = 0xcbf29ce4;
	let lo = 0x84222325;
	for (let i = 0; i < s.length; i++) {
		lo = (lo ^ s.charCodeAt(i)) >>> 0;
		// kali 0x100000001b3 = kali 0x1b3 ditambah (nilai << 40)
		const l = lo * 0x1b3;
		hi = (hi * 0x1b3 + Math.floor(l / 4294967296) + ((lo << 8) >>> 0)) >>> 0;
		lo = l >>> 0;
	}
	return hi.toString(16).padStart(8, '0') + lo.toString(16).padStart(8, '0');
}

/* ---------- Keadaan mesin ---------- */

const KUNCI_SINKRON = 'ngajio:sync:v1';
const BATCH = 500;
const DEBOUNCE_MS = 1200;
const INTERVAL_MS = 60_000;
const MAKS_ULANG = 5;
const TABEL_LIST: Exclude<Tabel, 'pengajar'>[] = ['metode', 'kelompok', 'santri', 'setoran', 'target'];
/** Tabel yang penghapusannya dikirim sebagai hapus:true. */
const BISA_HAPUS: Tabel[] = ['metode', 'target'];

let host: Host | null = null;
let st: Simpanan | null = null;
let gen = 0;
let jalan = false;
let ulang = false;
let gagalN = 0;
let gagal = false;
let sesiBerakhir = false;
let tDebounce: ReturnType<typeof setTimeout> | undefined;
let tUlang: ReturnType<typeof setTimeout> | undefined;
let tInterval: ReturnType<typeof setInterval> | undefined;

const uid = () =>
	typeof crypto !== 'undefined' && 'randomUUID' in crypto
		? crypto.randomUUID()
		: Math.random().toString(36).slice(2) + Date.now().toString(36);

function muatSimpanan(): Simpanan | null {
	try {
		const raw = localStorage.getItem(KUNCI_SINKRON);
		if (!raw) return null;
		const d = JSON.parse(raw) as Simpanan;
		return d.v === 1 && typeof d.sub === 'string' && Array.isArray(d.outbox) ? d : null;
	} catch {
		return null;
	}
}
function simpan() {
	if (!st) return;
	try {
		localStorage.setItem(KUNCI_SINKRON, JSON.stringify(st));
	} catch {
		/* penyimpanan penuh */
	}
}
function lapor() {
	host?.status({
		antrian: st?.outbox.length ?? 0,
		sibuk: jalan,
		gagal,
		sesiBerakhir,
		tglSinkron: st?.tglSinkron ?? null
	});
}

/** Ubah satu record store menjadi {id, data} sesuai pemetaan di docs/API.md. */
function pecah(t: Tabel, rec: object): { id: string; data: Record<string, unknown> } {
	if (t === 'pengajar') {
		// eslint-disable-next-line @typescript-eslint/no-unused-vars
		const { email, plan, ...data } = rec as Record<string, unknown>;
		return { id: 'me', data };
	}
	const { id, ...data } = rec as Record<string, unknown>;
	return { id: String(id), data };
}

/** Bandingkan snapshot dengan shadow, gabungkan selisih ke outbox (satu entri per record). */
function kumpulkan(d: Data) {
	if (!st) return;
	const idx = new Map<string, number>();
	st.outbox.forEach((e, i) => idx.set(e.t + '/' + e.id, i));
	const taruh = (e: Entri) => {
		const k = e.t + '/' + e.id;
		const i = idx.get(k);
		if (i == null) {
			idx.set(k, st!.outbox.length);
			st!.outbox.push(e);
		} else st!.outbox[i] = e;
	};
	const ada = new Set<string>();
	const proses = (t: Tabel, rec: object) => {
		const { id, data } = pecah(t, rec);
		const k = t + '/' + id;
		ada.add(k);
		const h = hash(data);
		if (st!.shadow[k] === h) return;
		st!.shadow[k] = h;
		taruh({ mid: uid(), t, id, data: JSON.parse(JSON.stringify(data)) });
	};
	proses('pengajar', d.pengajar);
	for (const t of TABEL_LIST) for (const r of d[t]) proses(t, r);
	for (const k of Object.keys(st.shadow)) {
		const t = k.slice(0, k.indexOf('/')) as Tabel;
		if (!BISA_HAPUS.includes(t) || ada.has(k)) continue;
		delete st.shadow[k];
		taruh({ mid: uid(), t, id: k.slice(k.indexOf('/') + 1), hapus: true });
	}
}

function terapkanTarikan(changes: Perubahan[]) {
	if (!st || !changes.length) return;
	const tertunda = new Set(st.outbox.map((e) => e.t + '/' + e.id));
	const pakai = changes.filter((c) => !tertunda.has(c.t + '/' + c.id));
	if (!pakai.length) return;
	host?.terapkan(pakai);
	for (const c of pakai) {
		const k = c.t + '/' + c.id;
		if (c.hapus) delete st.shadow[k];
		else st.shadow[k] = hash(c.data ?? {});
	}
}

async function flush() {
	if (!host || !st || sesiBerakhir) return;
	if (typeof navigator !== 'undefined' && !navigator.onLine) return;
	if (jalan) {
		ulang = true;
		return;
	}
	jalan = true;
	clearTimeout(tUlang);
	const g = gen;
	let ok = false;
	lapor();
	try {
		kumpulkan(host.potret());
		for (let i = 0; i < 1000; i++) {
			const batch = st.outbox.slice(0, BATCH);
			const r = await api<BalasanSinkron>('/api/sync', { cursor: st.cursor, changes: batch });
			if (g !== gen || !st) return;
			const terkirim = new Set(batch.map((e) => e.mid));
			st.outbox = st.outbox.filter((e) => !terkirim.has(e.mid));
			if (r.rejected?.length) {
				// Shadow sudah berisi hash lokal, jadi entri yang ditolak tidak dikirim lagi.
				host.toast(`${r.rejected.length} perubahan ditolak server dan dilewati`);
			}
			terapkanTarikan(r.changes ?? []);
			st.cursor = r.cursor;
			simpan();
			lapor();
			if (!r.more && st.outbox.length === 0) break;
		}
		ok = true;
		gagalN = 0;
		gagal = false;
		st.tglSinkron = new Date().toISOString();
	} catch (e) {
		if (g !== gen) return;
		if (e instanceof ApiError && e.status === 401) sesiBerakhir = true;
		else {
			gagalN++;
			if (gagalN <= MAKS_ULANG) tUlang = setTimeout(flush, 1000 * 2 ** gagalN);
			else gagal = true;
		}
	} finally {
		if (g === gen) {
			jalan = false;
			simpan();
			lapor();
			if (ok && ulang) {
				ulang = false;
				flush();
			}
		}
	}
}

/* ---------- API publik ---------- */

/** Mulai mesin untuk akun `sub`. Aman dipanggil ulang. */
export function mulai(h: Host, sub: string) {
	berhenti();
	host = h;
	st = muatSimpanan();
	if (!st || st.sub !== sub) st = { v: 1, sub, cursor: 0, shadow: {}, outbox: [], tglSinkron: null };
	sesiBerakhir = false;
	gagal = false;
	gagalN = 0;
	lapor();
	tInterval = setInterval(() => {
		if (document.visibilityState === 'visible') flush();
	}, INTERVAL_MS);
	flush();
}

export function berhenti() {
	gen++;
	clearTimeout(tDebounce);
	clearTimeout(tUlang);
	clearInterval(tInterval);
	jalan = false;
	ulang = false;
	st = null;
}

/** Hapus keadaan sinkron di penyimpanan (saat keluar). */
export function hapusSimpanan() {
	berhenti();
	try {
		localStorage.removeItem(KUNCI_SINKRON);
	} catch {
		/* penyimpanan tidak tersedia */
	}
}

/** Dipanggil store setiap ubah(): hitung selisih sekarang, kirim sesudah jeda. */
export function ubahan(d: Data) {
	if (!st) return;
	kumpulkan(d);
	simpan();
	lapor();
	clearTimeout(tDebounce);
	tDebounce = setTimeout(flush, DEBOUNCE_MS);
}

/** Pemicu: event online, aplikasi tampil lagi. */
export function picu() {
	flush();
}

export function cobaLagi() {
	gagalN = 0;
	gagal = false;
	lapor();
	flush();
}

/** Tarik semua record dari cursor 0 tanpa mengubah keadaan (untuk masukAkun). */
export async function tarikSemua(): Promise<{ cursor: number; changes: Perubahan[] }> {
	let cursor = 0;
	const changes: Perubahan[] = [];
	for (let i = 0; i < 1000; i++) {
		const r = await api<BalasanSinkron>('/api/sync', { cursor, changes: [] });
		changes.push(...(r.changes ?? []));
		cursor = r.cursor;
		if (!r.more) break;
	}
	return { cursor, changes };
}

/** Tulis keadaan awal untuk akun `sub` dari hasil tarik pertama (shadow = hash tarikan). */
export function inisialisasi(sub: string, cursor: number, changes: Perubahan[]) {
	const shadow: Record<string, string> = {};
	for (const c of changes) if (!c.hapus) shadow[c.t + '/' + c.id] = hash(c.data ?? {});
	st = { v: 1, sub, cursor, shadow, outbox: [], tglSinkron: null };
	simpan();
	st = null;
}
