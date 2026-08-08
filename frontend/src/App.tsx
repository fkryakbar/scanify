import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import scanifyLogo from './assets/images/scanify-logo.png';
import './App.css';
import {
    CheckForUpdate,
    DeletePage,
    DownloadUpdate,
    ExportSelected,
    GetSession,
    ListScanners,
    Scan,
    SetPageSelected,
} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';

type Operation = 'loading' | 'refreshing' | 'scanning' | 'exporting-jpg' | 'exporting-pdf' | 'downloading-update' | null;
type ToastKind = 'success' | 'warning' | 'error';

interface Toast {
    id: number;
    kind: ToastKind;
    title: string;
    message: string;
}

const emptySession = new main.SessionDTO({pages: [], selectedCount: 0, status: 'Mencari scanner WIA...'});

const modes = [
    {id: 'color', label: 'Warna', description: 'Dokumen penuh warna'},
    {id: 'grayscale', label: 'Abu-abu', description: 'Teks dan foto monokrom'},
    {id: 'blackwhite', label: 'Hitam putih', description: 'Dokumen teks tajam'},
] as const;

const resolutions = [150, 300, 600] as const;

function App() {
    const [scanners, setScanners] = useState<main.ScannerDTO[]>([]);
    const [selectedScanner, setSelectedScanner] = useState('');
    const [mode, setMode] = useState<(typeof modes)[number]['id']>('color');
    const [dpi, setDpi] = useState<(typeof resolutions)[number]>(300);
    const [session, setSession] = useState<main.SessionDTO>(emptySession);
    const [operation, setOperation] = useState<Operation>('loading');
    const [pendingPage, setPendingPage] = useState<string | null>(null);
    const [exportOpen, setExportOpen] = useState(false);
    const [updateInfo, setUpdateInfo] = useState<main.UpdateInfoDTO | null>(null);
    const [toasts, setToasts] = useState<Toast[]>([]);
    const toastID = useRef(0);

    const addToast = useCallback((kind: ToastKind, title: string, message: string) => {
        const id = ++toastID.current;
        setToasts((current) => [...current, {id, kind, title, message}]);
        window.setTimeout(() => {
            setToasts((current) => current.filter((toast) => toast.id !== id));
        }, 5000);
    }, []);

    const refreshScanners = useCallback(async (initial = false) => {
        if (!initial) setOperation('refreshing');
        try {
            const available = await ListScanners();
            setScanners(available ?? []);
            setSelectedScanner((current) => {
                if (available?.some((scanner) => scanner.id === current)) return current;
                return available?.[0]?.id ?? '';
            });
            if (!initial) {
                addToast(
                    available?.length ? 'success' : 'warning',
                    available?.length ? 'Scanner ditemukan' : 'Scanner tidak ditemukan',
                    available?.length
                        ? `${available.length} perangkat WIA siap digunakan.`
                        : 'Periksa kabel USB dan driver WIA, lalu coba segarkan kembali.',
                );
            }
        } catch (error) {
            setScanners([]);
            setSelectedScanner('');
            addToast('error', 'Gagal membaca scanner', errorMessage(error));
        } finally {
            if (!initial) setOperation(null);
        }
    }, [addToast]);

    useEffect(() => {
        let active = true;
        Promise.allSettled([GetSession(), ListScanners()]).then(([sessionResult, scannerResult]) => {
            if (!active) return;
            if (sessionResult.status === 'fulfilled') {
                setSession(sessionResult.value);
            } else {
                addToast('error', 'Sesi tidak dapat dimuat', errorMessage(sessionResult.reason));
            }
            if (scannerResult.status === 'fulfilled') {
                const available = scannerResult.value ?? [];
                setScanners(available);
                setSelectedScanner(available[0]?.id ?? '');
            } else {
                addToast('error', 'Scanner tidak dapat dibaca', errorMessage(scannerResult.reason));
            }
            setOperation(null);
        });
        return () => {
            active = false;
        };
    }, [addToast]);

    useEffect(() => {
        let active = true;
        CheckForUpdate()
            .then((info) => {
                if (active && info.available) setUpdateInfo(info);
            })
            .catch(() => {
                // Pemeriksaan otomatis tidak boleh mengganggu penggunaan saat sedang offline.
            });
        return () => {
            active = false;
        };
    }, []);

    const busy = operation !== null;
    const activeScannerName = useMemo(
        () => scanners.find((scanner) => scanner.id === selectedScanner)?.name,
        [scanners, selectedScanner],
    );

    async function handleScan() {
        if (!selectedScanner) {
            addToast('warning', 'Pilih scanner', 'Hubungkan dan pilih scanner sebelum memulai pemindaian.');
            return;
        }
        setOperation('scanning');
        try {
            const result = await Scan(new main.ScanRequest({scannerId: selectedScanner, mode, dpi}));
            setSession(result.session);
            addToast('success', 'Scan selesai', `Halaman ${result.session.pages.length} berhasil ditambahkan.`);
            if (result.warnings?.length) {
                addToast('warning', 'Peringatan driver', result.warnings.join(' '));
            }
        } catch (error) {
            addToast('error', 'Pemindaian gagal', errorMessage(error));
        } finally {
            setOperation(null);
        }
    }

    async function togglePage(page: main.PageDTO) {
        if (busy || pendingPage) return;
        setPendingPage(page.id);
        try {
            setSession(await SetPageSelected(page.id, !page.selected));
        } catch (error) {
            addToast('error', 'Pilihan gagal diperbarui', errorMessage(error));
        } finally {
            setPendingPage(null);
        }
    }

    async function removePage(page: main.PageDTO) {
        if (busy || pendingPage) return;
        setPendingPage(page.id);
        try {
            setSession(await DeletePage(page.id));
            addToast('success', 'Halaman dihapus', 'File sementara halaman telah dibersihkan.');
        } catch (error) {
            addToast('error', 'Halaman gagal dihapus', errorMessage(error));
        } finally {
            setPendingPage(null);
        }
    }

    async function handleExport(format: 'jpg' | 'pdf') {
        setExportOpen(false);
        setOperation(format === 'jpg' ? 'exporting-jpg' : 'exporting-pdf');
        try {
            const result = await ExportSelected(format);
            if (!result.cancelled) {
                addToast(
                    'success',
                    format === 'jpg' ? 'JPG berhasil disimpan' : 'PDF berhasil disimpan',
                    format === 'jpg'
                        ? `${result.paths.length} file disimpan sesuai urutan pilihan.`
                        : result.paths[0] ?? 'Dokumen selesai dibuat.',
                );
            }
        } catch (error) {
            addToast('error', 'Ekspor gagal', errorMessage(error));
        } finally {
            setOperation(null);
        }
    }

    async function handleDownloadUpdate() {
        if (!updateInfo || busy) return;
        setOperation('downloading-update');
        try {
            const result = await DownloadUpdate(updateInfo.latestVersion);
            setUpdateInfo(null);
            addToast(
                'success',
                `Scanify ${result.version} selesai diunduh`,
                `File baru disiapkan di ${result.path}. Scanify akan ditutup dan dibuka kembali otomatis.`,
            );
        } catch (error) {
            addToast('error', 'Pembaruan gagal diunduh', errorMessage(error));
        } finally {
            setOperation(null);
        }
    }

    return (
        <div className="app-shell">
            <header className="topbar">
                <div className="brand">
                    <img src={scanifyLogo} alt="" className="brand-logo"/>
                    <div>
                        <strong>Scanify</strong>
                        <span>Document workspace</span>
                    </div>
                </div>
                <div className={`connection-pill ${activeScannerName ? 'is-online' : ''}`}>
                    <span className="connection-dot"/>
                    <span>{activeScannerName ? `${activeScannerName} siap` : 'Scanner belum siap'}</span>
                </div>
            </header>

            <main className="main-layout">
                <aside className="settings-panel" aria-label="Pengaturan pemindaian">
                    <div className="panel-heading">
                        <div>
                            <span className="eyebrow">Pengaturan</span>
                            <h1>Pindai dokumen</h1>
                        </div>
                        <button
                            className="icon-button"
                            type="button"
                            aria-label="Segarkan daftar scanner"
                            title="Segarkan daftar scanner"
                            disabled={busy}
                            onClick={() => refreshScanners(false)}
                        >
                            <RefreshIcon/>
                        </button>
                    </div>

                    <label className="field-group">
                        <span className="field-label">Scanner</span>
                        <span className="select-wrap">
                            <select
                                value={selectedScanner}
                                disabled={busy || scanners.length === 0}
                                onChange={(event) => setSelectedScanner(event.target.value)}
                                aria-label="Pilih scanner"
                            >
                                {scanners.length === 0 && <option value="">Scanner tidak terdeteksi</option>}
                                {scanners.map((scanner) => (
                                    <option key={scanner.id} value={scanner.id}>{scanner.name}</option>
                                ))}
                            </select>
                            <ChevronIcon/>
                        </span>
                        <span className="field-help">
                            {scanners.length ? 'Perangkat WIA terhubung dan siap.' : 'Hubungkan scanner lalu tekan tombol segarkan.'}
                        </span>
                    </label>

                    <fieldset className="field-group">
                        <legend className="field-label">Mode warna</legend>
                        <div className="mode-list">
                            {modes.map((item) => (
                                <button
                                    key={item.id}
                                    type="button"
                                    className={`mode-option ${mode === item.id ? 'is-active' : ''}`}
                                    aria-pressed={mode === item.id}
                                    disabled={busy}
                                    onClick={() => setMode(item.id)}
                                >
                                    <span className={`mode-swatch mode-${item.id}`}/>
                                    <span>
                                        <strong>{item.label}</strong>
                                        <small>{item.description}</small>
                                    </span>
                                    <span className="radio-mark"/>
                                </button>
                            ))}
                        </div>
                    </fieldset>

                    <fieldset className="field-group">
                        <legend className="field-label">Resolusi</legend>
                        <div className="dpi-options">
                            {resolutions.map((value) => (
                                <button
                                    key={value}
                                    type="button"
                                    className={dpi === value ? 'is-active' : ''}
                                    aria-pressed={dpi === value}
                                    disabled={busy}
                                    onClick={() => setDpi(value)}
                                >
                                    <strong>{value}</strong>
                                    <span>DPI</span>
                                    {value === 300 && <small>Disarankan</small>}
                                </button>
                            ))}
                        </div>
                    </fieldset>

                    <div className="settings-footer">
                        <button
                            className="scan-button"
                            type="button"
                            onClick={handleScan}
                            disabled={busy || !selectedScanner}
                        >
                            <ScanIcon/>
                            <span>{operation === 'scanning' ? 'Sedang memindai...' : 'Mulai scan'}</span>
                        </button>
                        <div className="status-card" aria-live="polite">
                            <span className={`status-indicator ${activeScannerName ? 'is-ready' : ''}`}/>
                            <div>
                                <strong>Status</strong>
                                <span>{session.status || 'Siap menerima perintah.'}</span>
                            </div>
                        </div>
                    </div>
                </aside>

                <section className="workspace" aria-label="Halaman hasil scan">
                    <header className="workspace-heading">
                        <div>
                            <span className="eyebrow">Workspace</span>
                            <div className="title-row">
                                <h2>Halaman hasil scan</h2>
                                <span className="page-count">{session.pages.length}</span>
                            </div>
                            <p>{session.selectedCount ? `${session.selectedCount} halaman dipilih untuk diekspor` : 'Pilih halaman sesuai urutan ekspor'}</p>
                        </div>
                        <button
                            type="button"
                            className="save-button"
                            onClick={() => setExportOpen(true)}
                            disabled={busy || session.selectedCount === 0}
                        >
                            <DownloadIcon/>
                            <span>Simpan</span>
                            {session.selectedCount > 0 && <span className="selected-badge">{session.selectedCount}</span>}
                        </button>
                    </header>

                    <div className={`gallery-surface ${session.pages.length === 0 ? 'is-empty' : ''}`}>
                        {session.pages.length === 0 ? (
                            <div className="empty-state">
                                <div className="empty-illustration">
                                    <DocumentIcon/>
                                    <span className="scan-line"/>
                                </div>
                                <h3>Belum ada halaman</h3>
                                <p>Letakkan dokumen pada scanner, pilih pengaturan, lalu tekan <strong>Mulai scan</strong>.</p>
                            </div>
                        ) : (
                            <div className="page-grid">
                                {session.pages.map((page, index) => (
                                    <article
                                        key={page.id}
                                        className={`page-card ${page.selected ? 'is-selected' : ''} ${pendingPage === page.id ? 'is-pending' : ''}`}
                                    >
                                        <div className="page-card-toolbar">
                                            <button
                                                type="button"
                                                className="selection-button"
                                                aria-label={`${page.selected ? 'Batalkan pilihan' : 'Pilih'} halaman ${index + 1}`}
                                                aria-pressed={page.selected}
                                                disabled={busy || pendingPage !== null}
                                                onClick={() => togglePage(page)}
                                            >
                                                {page.selected && <CheckIcon/>}
                                            </button>
                                            {page.selected && <span className="order-badge" aria-label={`Urutan ekspor ${page.selectionOrder}`}>{page.selectionOrder}</span>}
                                            <button
                                                type="button"
                                                className="delete-button"
                                                aria-label={`Hapus halaman ${index + 1}`}
                                                disabled={busy || pendingPage !== null}
                                                onClick={() => removePage(page)}
                                            >
                                                <TrashIcon/>
                                            </button>
                                        </div>
                                        <button
                                            type="button"
                                            className="page-preview"
                                            aria-label={`${page.selected ? 'Batalkan pilihan melalui pratinjau' : 'Pilih melalui pratinjau'} halaman ${index + 1}`}
                                            disabled={busy || pendingPage !== null}
                                            onClick={() => togglePage(page)}
                                        >
                                            <img src={page.thumbnailDataURL} alt={`Pratinjau halaman ${index + 1}`}/>
                                        </button>
                                        <footer>
                                            <div>
                                                <strong>Halaman {index + 1}</strong>
                                                <span>{page.width} × {page.height} px</span>
                                            </div>
                                            <span className="file-chip">SCAN</span>
                                        </footer>
                                    </article>
                                ))}
                            </div>
                        )}

                        {busy && (
                            <div className="busy-overlay" role="status" aria-live="polite">
                                <span className="spinner"/>
                                <strong>{operationTitle(operation)}</strong>
                                <span>{operationDescription(operation)}</span>
                            </div>
                        )}
                    </div>
                </section>
            </main>

            {updateInfo && (
                <div className="modal-backdrop" role="presentation">
                    <section
                        className="export-modal update-modal"
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="update-title"
                    >
                        <div className="modal-icon"><DownloadIcon/></div>
                        <div className="modal-copy">
                            <span className="eyebrow">Pembaruan tersedia</span>
                            <h2 id="update-title">Update ke Scanify {updateInfo.latestVersion}?</h2>
                            <p>
                                Versi saat ini {updateInfo.currentVersion}. File resmi akan diunduh otomatis dari GitHub
                                dan diverifikasi dengan SHA-256.
                            </p>
                        </div>
                        {updateInfo.releaseNotes && (
                            <div className="release-notes">
                                <strong>{updateInfo.releaseName || `Scanify ${updateInfo.latestVersion}`}</strong>
                                <p>{updateInfo.releaseNotes}</p>
                            </div>
                        )}
                        <div className="update-actions">
                            <button
                                className="cancel-button"
                                type="button"
                                disabled={operation === 'downloading-update'}
                                onClick={() => setUpdateInfo(null)}
                            >
                                Nanti saja
                            </button>
                            <button
                                className="scan-button"
                                type="button"
                                disabled={busy}
                                onClick={handleDownloadUpdate}
                            >
                                <DownloadIcon/>
                                <span>{operation === 'downloading-update' ? 'Mengunduh...' : 'Ya, unduh update'}</span>
                            </button>
                        </div>
                    </section>
                </div>
            )}

            {exportOpen && (
                <div className="modal-backdrop" role="presentation" onMouseDown={() => setExportOpen(false)}>
                    <section
                        className="export-modal"
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="export-title"
                        onMouseDown={(event) => event.stopPropagation()}
                    >
                        <div className="modal-icon"><DownloadIcon/></div>
                        <div className="modal-copy">
                            <span className="eyebrow">Ekspor dokumen</span>
                            <h2 id="export-title">Pilih format penyimpanan</h2>
                            <p>{session.selectedCount} halaman akan disimpan sesuai urutan badge hijau.</p>
                        </div>
                        <div className="export-options">
                            <button type="button" onClick={() => handleExport('jpg')}>
                                <span className="format-icon">JPG</span>
                                <span><strong>Gambar JPG</strong><small>Satu file untuk setiap halaman</small></span>
                                <ChevronRightIcon/>
                            </button>
                            <button type="button" onClick={() => handleExport('pdf')}>
                                <span className="format-icon pdf">PDF</span>
                                <span><strong>Dokumen PDF</strong><small>Semua halaman dalam satu file</small></span>
                                <ChevronRightIcon/>
                            </button>
                        </div>
                        <button className="cancel-button" type="button" onClick={() => setExportOpen(false)}>Batal</button>
                    </section>
                </div>
            )}

            <div className="toast-region" aria-live="polite" aria-label="Notifikasi">
                {toasts.map((toast) => (
                    <article key={toast.id} className={`toast toast-${toast.kind}`}>
                        <span className="toast-symbol">{toast.kind === 'success' ? '✓' : toast.kind === 'warning' ? '!' : '×'}</span>
                        <div><strong>{toast.title}</strong><span>{toast.message}</span></div>
                        <button type="button" aria-label="Tutup notifikasi" onClick={() => setToasts((current) => current.filter((item) => item.id !== toast.id))}>×</button>
                    </article>
                ))}
            </div>
        </div>
    );
}

function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    if (typeof error === 'string') return error;
    return 'Terjadi kesalahan yang tidak diketahui.';
}

function operationTitle(operation: Operation): string {
    switch (operation) {
        case 'loading': return 'Menyiapkan Scanify';
        case 'refreshing': return 'Mencari scanner';
        case 'scanning': return 'Sedang memindai';
        case 'exporting-jpg': return 'Menyimpan gambar JPG';
        case 'exporting-pdf': return 'Menyusun dokumen PDF';
        case 'downloading-update': return 'Mengunduh pembaruan';
        default: return 'Memproses';
    }
}

function operationDescription(operation: Operation): string {
    switch (operation) {
        case 'scanning': return 'Jangan mengangkat penutup scanner hingga proses selesai.';
        case 'exporting-jpg': return 'Mengonversi halaman sesuai urutan pilihan.';
        case 'exporting-pdf': return 'Menata setiap halaman pada lembar A4.';
        case 'downloading-update': return 'Mengunduh file resmi dan memverifikasi SHA-256.';
        default: return 'Mohon tunggu sebentar.';
    }
}

function RefreshIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 11a8 8 0 1 0-2.34 5.66M20 4v7h-7"/></svg>; }
function ScanIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 4V2h12v2M5 18H3V8a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v10h-2M6 14h12v8H6z"/><path d="M17 10h.01"/></svg>; }
function DownloadIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12m0 0 5-5m-5 5-5-5M5 21h14"/></svg>; }
function DocumentIcon() { return <svg viewBox="0 0 64 64" aria-hidden="true"><path d="M18 6h20l10 10v42H18z"/><path d="M38 6v12h12M25 29h16M25 37h16M25 45h11"/></svg>; }
function CheckIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"/></svg>; }
function TrashIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6"/></svg>; }
function ChevronIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 10 4 4 4-4"/></svg>; }
function ChevronRightIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18 6-6-6-6"/></svg>; }

export default App;
