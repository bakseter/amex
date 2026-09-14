// components/export-bar.tsx
import { backendUrl } from '@/api/transaction';

const ExportBar = ({ invoiceId }: { invoiceId: number }) => {
    // backendUrl already ends in /api — adding it again was the old bug.
    const base = `${backendUrl}/invoices/${String(invoiceId)}`;

    return (
        <div className="flex shrink-0 items-center gap-4 border-t border-border px-5 py-2.5 text-xs text-muted2">
            <a
                href={`${base}/export/transactions`}
                download
                className="hover:text-text"
            >
                Download transactions
            </a>
            <a
                href={`${base}/export/summary`}
                download
                className="hover:text-text"
            >
                Download summary
            </a>
            <a
                href={`${base}/csv`}
                download
                className="ml-auto hover:text-text"
            >
                Original file
            </a>
        </div>
    );
};

export default ExportBar;
