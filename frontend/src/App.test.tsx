import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import React from 'react';
import {cleanup, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';
import * as bindings from '../wailsjs/go/main/App';

vi.mock('../wailsjs/go/main/App', () => ({
    CheckForUpdate: vi.fn(),
    DeletePage: vi.fn(),
    DownloadUpdate: vi.fn(),
    ExportSelected: vi.fn(),
    GetArsipinConfig: vi.fn(),
    GetSession: vi.fn(),
    ListScanners: vi.fn(),
    SaveArsipinConfig: vi.fn(),
    Scan: vi.fn(),
    SetPageSelected: vi.fn(),
    UploadSelectedToArsipin: vi.fn(),
}));

const baseSession = {
    pages: [],
    selectedCount: 0,
    status: '1 scanner siap digunakan.',
};

describe('Scanify workspace', () => {
    beforeEach(() => {
        vi.mocked(bindings.CheckForUpdate).mockResolvedValue({
            available: false,
            currentVersion: '2.0.0',
            latestVersion: '2.0.0',
            releaseName: '',
            releaseNotes: '',
        } as any);
        vi.mocked(bindings.GetSession).mockResolvedValue(baseSession as any);
        vi.mocked(bindings.ListScanners).mockResolvedValue([{id: 'canon-1', name: 'WIA Canon MP280 ser'}] as any);
        vi.mocked(bindings.GetArsipinConfig).mockResolvedValue({uploadUrl: '', passwordConfigured: false} as any);
    });

    afterEach(() => {
        cleanup();
        vi.clearAllMocks();
    });

    it('memilih scanner WIA pertama dan menampilkan empty state', async () => {
        render(<App/>);

        expect(await screen.findByRole('option', {name: 'WIA Canon MP280 ser'})).toBeInTheDocument();
        expect(screen.getByText('Belum ada halaman')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Mulai scan'})).toBeEnabled();
    });

    it('menampilkan keadaan tanpa scanner secara jelas', async () => {
        vi.mocked(bindings.ListScanners).mockResolvedValue([]);
        render(<App/>);

        expect(await screen.findByRole('option', {name: 'Scanner tidak terdeteksi'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Mulai scan'})).toBeDisabled();
    });

    it('memperbarui badge urutan ketika halaman dipilih', async () => {
        const page = {
            id: 'page-1', thumbnailDataURL: 'data:image/jpeg;base64,AA==', selected: false,
            selectionOrder: 0, width: 1200, height: 1700,
        };
        vi.mocked(bindings.GetSession).mockResolvedValue({...baseSession, pages: [page]} as any);
        vi.mocked(bindings.SetPageSelected).mockResolvedValue({
            ...baseSession,
            pages: [{...page, selected: true, selectionOrder: 1}],
            selectedCount: 1,
        } as any);

        render(<App/>);
        const user = userEvent.setup();
        await user.click(await screen.findByRole('button', {name: 'Pilih halaman 1'}));

        await waitFor(() => expect(bindings.SetPageSelected).toHaveBeenCalledWith('page-1', true));
        expect(await screen.findByLabelText('Urutan ekspor 1')).toHaveTextContent('1');
        expect(screen.getByRole('button', {name: /^Simpan/})).toBeEnabled();
    });

    it('menahan upload tanpa konfigurasi dan membuka modal melalui ikon gear', async () => {
        const page = {
            id: 'page-1', thumbnailDataURL: 'data:image/jpeg;base64,AA==', selected: true,
            selectionOrder: 1, width: 1200, height: 1700,
        };
        vi.mocked(bindings.GetSession).mockResolvedValue({...baseSession, pages: [page], selectedCount: 1} as any);

        render(<App/>);
        const user = userEvent.setup();

        expect(await screen.findByRole('button', {name: 'Upload ke Arsipin'})).toBeDisabled();
        await user.click(screen.getByRole('button', {name: 'Konfigurasi Arsipin'}));
        expect(screen.getByRole('dialog', {name: 'Konfigurasi upload'})).toBeInTheDocument();
    });

    it('mengunggah halaman terpilih dan menampilkan job Arsipin', async () => {
        const page = {
            id: 'page-1', thumbnailDataURL: 'data:image/jpeg;base64,AA==', selected: true,
            selectionOrder: 1, width: 1200, height: 1700,
        };
        vi.mocked(bindings.GetSession).mockResolvedValue({...baseSession, pages: [page], selectedCount: 1} as any);
        vi.mocked(bindings.GetArsipinConfig).mockResolvedValue({uploadUrl: 'https://arsipin.example/upload', passwordConfigured: true} as any);
        vi.mocked(bindings.UploadSelectedToArsipin).mockResolvedValue({
            success: true,
            message: 'Dokumen masuk antrean pemrosesan.',
            statusCode: 202,
            errorCode: '',
            archiveId: '01AR',
            jobId: '01JB',
        } as any);

        render(<App/>);
        const user = userEvent.setup();
        await user.click(await screen.findByRole('button', {name: 'Upload ke Arsipin'}));

        await waitFor(() => expect(bindings.UploadSelectedToArsipin).toHaveBeenCalledTimes(1));
        expect(await screen.findByText('Upload ke Arsipin berhasil')).toBeInTheDocument();
        expect(await screen.findByText(/01JB/)).toBeInTheDocument();
    });

    it('meminta persetujuan lalu mengunduh update yang dipilih', async () => {
        vi.mocked(bindings.CheckForUpdate).mockResolvedValue({
            available: true,
            currentVersion: '2.0.0',
            latestVersion: '2.1.0',
            releaseName: 'Scanify 2.1.0',
            releaseNotes: 'Perbaikan stabilitas.',
        } as any);
        vi.mocked(bindings.DownloadUpdate).mockResolvedValue({
            version: '2.1.0',
            path: 'C:\\Users\\tester\\Downloads\\Scanify-v2.1.0-windows-amd64.exe',
        } as any);

        render(<App/>);
        const user = userEvent.setup();
        expect(await screen.findByRole('dialog', {name: 'Update ke Scanify 2.1.0?'})).toBeInTheDocument();
        await user.click(screen.getByRole('button', {name: 'Ya, unduh update'}));

        await waitFor(() => expect(bindings.DownloadUpdate).toHaveBeenCalledWith('2.1.0'));
        expect(await screen.findByText('Scanify 2.1.0 selesai diunduh')).toBeInTheDocument();
        expect(screen.queryByRole('dialog', {name: 'Update ke Scanify 2.1.0?'})).not.toBeInTheDocument();
    });
});
