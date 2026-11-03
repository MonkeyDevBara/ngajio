export type Surah = {
	n: number;
	latin: string;
	arab: string;
	ayat: number;
	juz: number;
};

/**
 * Metadata 114 surat. Jumlah ayat mengikuti hitungan Kufi (total 6236).
 * Juz di sini adalah juz tempat surat dimulai.
 */
export const SURAHS: Surah[] = [
	{ n: 1, latin: 'Al-Fatihah', arab: 'الفاتحة', ayat: 7, juz: 1 },
	{ n: 2, latin: 'Al-Baqarah', arab: 'البقرة', ayat: 286, juz: 1 },
	{ n: 3, latin: 'Ali Imran', arab: 'آل عمران', ayat: 200, juz: 3 },
	{ n: 4, latin: 'An-Nisa', arab: 'النساء', ayat: 176, juz: 4 },
	{ n: 5, latin: "Al-Ma'idah", arab: 'المائدة', ayat: 120, juz: 6 },
	{ n: 6, latin: "Al-An'am", arab: 'الأنعام', ayat: 165, juz: 7 },
	{ n: 7, latin: "Al-A'raf", arab: 'الأعراف', ayat: 206, juz: 8 },
	{ n: 8, latin: 'Al-Anfal', arab: 'الأنفال', ayat: 75, juz: 9 },
	{ n: 9, latin: 'At-Taubah', arab: 'التوبة', ayat: 129, juz: 10 },
	{ n: 10, latin: 'Yunus', arab: 'يونس', ayat: 109, juz: 11 },
	{ n: 11, latin: 'Hud', arab: 'هود', ayat: 123, juz: 11 },
	{ n: 12, latin: 'Yusuf', arab: 'يوسف', ayat: 111, juz: 12 },
	{ n: 13, latin: "Ar-Ra'd", arab: 'الرعد', ayat: 43, juz: 13 },
	{ n: 14, latin: 'Ibrahim', arab: 'إبراهيم', ayat: 52, juz: 13 },
	{ n: 15, latin: 'Al-Hijr', arab: 'الحجر', ayat: 99, juz: 14 },
	{ n: 16, latin: 'An-Nahl', arab: 'النحل', ayat: 128, juz: 14 },
	{ n: 17, latin: "Al-Isra", arab: 'الإسراء', ayat: 111, juz: 15 },
	{ n: 18, latin: 'Al-Kahfi', arab: 'الكهف', ayat: 110, juz: 15 },
	{ n: 19, latin: 'Maryam', arab: 'مريم', ayat: 98, juz: 16 },
	{ n: 20, latin: 'Taha', arab: 'طه', ayat: 135, juz: 16 },
	{ n: 21, latin: 'Al-Anbiya', arab: 'الأنبياء', ayat: 112, juz: 17 },
	{ n: 22, latin: 'Al-Hajj', arab: 'الحج', ayat: 78, juz: 17 },
	{ n: 23, latin: "Al-Mu'minun", arab: 'المؤمنون', ayat: 118, juz: 18 },
	{ n: 24, latin: 'An-Nur', arab: 'النور', ayat: 64, juz: 18 },
	{ n: 25, latin: 'Al-Furqan', arab: 'الفرقان', ayat: 77, juz: 18 },
	{ n: 26, latin: "Asy-Syu'ara", arab: 'الشعراء', ayat: 227, juz: 19 },
	{ n: 27, latin: 'An-Naml', arab: 'النمل', ayat: 93, juz: 19 },
	{ n: 28, latin: 'Al-Qasas', arab: 'القصص', ayat: 88, juz: 20 },
	{ n: 29, latin: 'Al-Ankabut', arab: 'العنكبوت', ayat: 69, juz: 20 },
	{ n: 30, latin: 'Ar-Rum', arab: 'الروم', ayat: 60, juz: 21 },
	{ n: 31, latin: 'Luqman', arab: 'لقمان', ayat: 34, juz: 21 },
	{ n: 32, latin: 'As-Sajdah', arab: 'السجدة', ayat: 30, juz: 21 },
	{ n: 33, latin: 'Al-Ahzab', arab: 'الأحزاب', ayat: 73, juz: 21 },
	{ n: 34, latin: 'Saba', arab: 'سبأ', ayat: 54, juz: 22 },
	{ n: 35, latin: 'Fatir', arab: 'فاطر', ayat: 45, juz: 22 },
	{ n: 36, latin: 'Yasin', arab: 'يس', ayat: 83, juz: 22 },
	{ n: 37, latin: 'As-Saffat', arab: 'الصافات', ayat: 182, juz: 23 },
	{ n: 38, latin: 'Sad', arab: 'ص', ayat: 88, juz: 23 },
	{ n: 39, latin: 'Az-Zumar', arab: 'الزمر', ayat: 75, juz: 23 },
	{ n: 40, latin: 'Gafir', arab: 'غافر', ayat: 85, juz: 24 },
	{ n: 41, latin: 'Fussilat', arab: 'فصلت', ayat: 54, juz: 24 },
	{ n: 42, latin: 'Asy-Syura', arab: 'الشورى', ayat: 53, juz: 25 },
	{ n: 43, latin: 'Az-Zukhruf', arab: 'الزخرف', ayat: 89, juz: 25 },
	{ n: 44, latin: 'Ad-Dukhan', arab: 'الدخان', ayat: 59, juz: 25 },
	{ n: 45, latin: 'Al-Jasiyah', arab: 'الجاثية', ayat: 37, juz: 25 },
	{ n: 46, latin: 'Al-Ahqaf', arab: 'الأحقاف', ayat: 35, juz: 26 },
	{ n: 47, latin: 'Muhammad', arab: 'محمد', ayat: 38, juz: 26 },
	{ n: 48, latin: 'Al-Fath', arab: 'الفتح', ayat: 29, juz: 26 },
	{ n: 49, latin: 'Al-Hujurat', arab: 'الحجرات', ayat: 18, juz: 26 },
	{ n: 50, latin: 'Qaf', arab: 'ق', ayat: 45, juz: 26 },
	{ n: 51, latin: 'Az-Zariyat', arab: 'الذاريات', ayat: 60, juz: 26 },
	{ n: 52, latin: 'At-Tur', arab: 'الطور', ayat: 49, juz: 27 },
	{ n: 53, latin: 'An-Najm', arab: 'النجم', ayat: 62, juz: 27 },
	{ n: 54, latin: 'Al-Qamar', arab: 'القمر', ayat: 55, juz: 27 },
	{ n: 55, latin: 'Ar-Rahman', arab: 'الرحمن', ayat: 78, juz: 27 },
	{ n: 56, latin: "Al-Waqi'ah", arab: 'الواقعة', ayat: 96, juz: 27 },
	{ n: 57, latin: 'Al-Hadid', arab: 'الحديد', ayat: 29, juz: 27 },
	{ n: 58, latin: 'Al-Mujadalah', arab: 'المجادلة', ayat: 22, juz: 28 },
	{ n: 59, latin: 'Al-Hasyr', arab: 'الحشر', ayat: 24, juz: 28 },
	{ n: 60, latin: 'Al-Mumtahanah', arab: 'الممتحنة', ayat: 13, juz: 28 },
	{ n: 61, latin: 'As-Saff', arab: 'الصف', ayat: 14, juz: 28 },
	{ n: 62, latin: "Al-Jumu'ah", arab: 'الجمعة', ayat: 11, juz: 28 },
	{ n: 63, latin: 'Al-Munafiqun', arab: 'المنافقون', ayat: 11, juz: 28 },
	{ n: 64, latin: 'At-Tagabun', arab: 'التغابن', ayat: 18, juz: 28 },
	{ n: 65, latin: 'At-Talaq', arab: 'الطلاق', ayat: 12, juz: 28 },
	{ n: 66, latin: 'At-Tahrim', arab: 'التحريم', ayat: 12, juz: 28 },
	{ n: 67, latin: 'Al-Mulk', arab: 'الملك', ayat: 30, juz: 29 },
	{ n: 68, latin: 'Al-Qalam', arab: 'القلم', ayat: 52, juz: 29 },
	{ n: 69, latin: 'Al-Haqqah', arab: 'الحاقة', ayat: 52, juz: 29 },
	{ n: 70, latin: "Al-Ma'arij", arab: 'المعارج', ayat: 44, juz: 29 },
	{ n: 71, latin: 'Nuh', arab: 'نوح', ayat: 28, juz: 29 },
	{ n: 72, latin: 'Al-Jinn', arab: 'الجن', ayat: 28, juz: 29 },
	{ n: 73, latin: 'Al-Muzzammil', arab: 'المزمل', ayat: 20, juz: 29 },
	{ n: 74, latin: 'Al-Muddassir', arab: 'المدثر', ayat: 56, juz: 29 },
	{ n: 75, latin: 'Al-Qiyamah', arab: 'القيامة', ayat: 40, juz: 29 },
	{ n: 76, latin: 'Al-Insan', arab: 'الإنسان', ayat: 31, juz: 29 },
	{ n: 77, latin: 'Al-Mursalat', arab: 'المرسلات', ayat: 50, juz: 29 },
	{ n: 78, latin: "An-Naba'", arab: 'النبأ', ayat: 40, juz: 30 },
	{ n: 79, latin: "An-Nazi'at", arab: 'النازعات', ayat: 46, juz: 30 },
	{ n: 80, latin: 'Abasa', arab: 'عبس', ayat: 42, juz: 30 },
	{ n: 81, latin: 'At-Takwir', arab: 'التكوير', ayat: 29, juz: 30 },
	{ n: 82, latin: 'Al-Infitar', arab: 'الانفطار', ayat: 19, juz: 30 },
	{ n: 83, latin: 'Al-Mutaffifin', arab: 'المطففين', ayat: 36, juz: 30 },
	{ n: 84, latin: 'Al-Insyiqaq', arab: 'الانشقاق', ayat: 25, juz: 30 },
	{ n: 85, latin: 'Al-Buruj', arab: 'البروج', ayat: 22, juz: 30 },
	{ n: 86, latin: 'At-Tariq', arab: 'الطارق', ayat: 17, juz: 30 },
	{ n: 87, latin: "Al-A'la", arab: 'الأعلى', ayat: 19, juz: 30 },
	{ n: 88, latin: 'Al-Gasyiyah', arab: 'الغاشية', ayat: 26, juz: 30 },
	{ n: 89, latin: 'Al-Fajr', arab: 'الفجر', ayat: 30, juz: 30 },
	{ n: 90, latin: 'Al-Balad', arab: 'البلد', ayat: 20, juz: 30 },
	{ n: 91, latin: 'Asy-Syams', arab: 'الشمس', ayat: 15, juz: 30 },
	{ n: 92, latin: 'Al-Lail', arab: 'الليل', ayat: 21, juz: 30 },
	{ n: 93, latin: 'Ad-Duha', arab: 'الضحى', ayat: 11, juz: 30 },
	{ n: 94, latin: 'Asy-Syarh', arab: 'الشرح', ayat: 8, juz: 30 },
	{ n: 95, latin: 'At-Tin', arab: 'التين', ayat: 8, juz: 30 },
	{ n: 96, latin: "Al-'Alaq", arab: 'العلق', ayat: 19, juz: 30 },
	{ n: 97, latin: 'Al-Qadr', arab: 'القدر', ayat: 5, juz: 30 },
	{ n: 98, latin: 'Al-Bayyinah', arab: 'البينة', ayat: 8, juz: 30 },
	{ n: 99, latin: 'Az-Zalzalah', arab: 'الزلزلة', ayat: 8, juz: 30 },
	{ n: 100, latin: "Al-'Adiyat", arab: 'العاديات', ayat: 11, juz: 30 },
	{ n: 101, latin: "Al-Qari'ah", arab: 'القارعة', ayat: 11, juz: 30 },
	{ n: 102, latin: 'At-Takasur', arab: 'التكاثر', ayat: 8, juz: 30 },
	{ n: 103, latin: "Al-'Asr", arab: 'العصر', ayat: 3, juz: 30 },
	{ n: 104, latin: 'Al-Humazah', arab: 'الهمزة', ayat: 9, juz: 30 },
	{ n: 105, latin: 'Al-Fil', arab: 'الفيل', ayat: 5, juz: 30 },
	{ n: 106, latin: 'Quraisy', arab: 'قريش', ayat: 4, juz: 30 },
	{ n: 107, latin: "Al-Ma'un", arab: 'الماعون', ayat: 7, juz: 30 },
	{ n: 108, latin: 'Al-Kausar', arab: 'الكوثر', ayat: 3, juz: 30 },
	{ n: 109, latin: 'Al-Kafirun', arab: 'الكافرون', ayat: 6, juz: 30 },
	{ n: 110, latin: 'An-Nasr', arab: 'النصر', ayat: 3, juz: 30 },
	{ n: 111, latin: 'Al-Lahab', arab: 'اللهب', ayat: 5, juz: 30 },
	{ n: 112, latin: 'Al-Ikhlas', arab: 'الإخلاص', ayat: 4, juz: 30 },
	{ n: 113, latin: 'Al-Falaq', arab: 'الفلق', ayat: 5, juz: 30 },
	{ n: 114, latin: 'An-Nas', arab: 'الناس', ayat: 6, juz: 30 }
];

export const byNumber = new Map(SURAHS.map((s) => [s.n, s]));

export function surah(n: number): Surah {
	const s = byNumber.get(n);
	if (!s) throw new Error(`Surat ${n} tidak ada`);
	return s;
}

/** Urutan Juz 30 dari An-Nas turun ke An-Naba' (arah hafalan pemula). */
export const JUZ30_REVERSE = SURAHS.filter((s) => s.n >= 78).reverse();

/** Normalisasi untuk pencarian longgar: "Al-Ikhlas", "Ikhlas", "Ihlas" semua cocok. */
export function normalize(text: string): string {
	return text
		.toLowerCase()
		.replace(/['’`ʿʾ]/g, '')
		.replace(/al-/g, '')
		.replace(/([aeiou])h/g, '$1')
		.replace(/[^a-z0-9]/g, '');
}

const NORMALIZED = SURAHS.map((s) => ({ s, key: normalize(s.latin) }));

export function searchSurah(query: string, limit = 8): Surah[] {
	const q = normalize(query);
	if (!q) return [];
	return NORMALIZED.filter(({ key }) => key.startsWith(q) || key.includes(q))
		.sort((a, b) => Number(b.key.startsWith(q)) - Number(a.key.startsWith(q)) || a.s.n - b.s.n)
		.slice(0, limit)
		.map(({ s }) => s);
}

export function ayatLabel(n: number, from: number, to: number): string {
	return from === to ? `Ayat ${from}` : `Ayat ${from}–${to}`;
}

export function positionLabel(n: number, ayat: number): string {
	const s = surah(n);
	return `${s.latin} : ${ayat}`;
}
