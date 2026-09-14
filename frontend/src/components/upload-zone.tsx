// components/upload-zone.tsx
import { useRef, useState } from 'react';

import { useUploadInvoiceMutation } from '@/api/transaction';

const UploadZone = ({ onUploaded }: { onUploaded: (id: number) => void }) => {
    const [upload, { isLoading }] = useUploadInvoiceMutation();
    const [dragging, setDragging] = useState(false);
    const [error, setError] = useState('');
    const ref = useRef<HTMLInputElement>(null);

    const handle = async (file: File | undefined) => {
        if (!file) {
            return;
        }

        if (!file.name.toLowerCase().endsWith('.csv')) {
            setError(
                'That is not a CSV. Export the statement as CSV from Amex.'
            );

            return;
        }

        setError('');

        try {
            // The upload response names it invoice_id, not id.
            const result = await upload(file).unwrap();

            onUploaded(result.invoice_id);
        } catch {
            setError('Upload failed. Check the file is an Amex CSV export.');
        }
    };

    return (
        <div className="flex flex-col gap-1.5">
            <div
                onClick={() => ref.current?.click()}
                onDragOver={(event) => {
                    event.preventDefault();
                    setDragging(true);
                }}
                onDragLeave={() => {
                    setDragging(false);
                }}
                onDrop={(event) => {
                    event.preventDefault();
                    setDragging(false);
                    void handle(event.dataTransfer.files[0]);
                }}
                className={`cursor-pointer rounded-lg border border-dashed px-4 py-5 text-center text-sm transition-colors ${
                    dragging
                        ? 'border-andreas bg-surface2'
                        : 'border-border2 text-muted2 hover:border-muted2 hover:bg-surface2'
                }`}
            >
                {isLoading ? 'Reading statement…' : 'Drop a statement CSV'}
                <input
                    ref={ref}
                    type="file"
                    accept=".csv,text/csv"
                    className="hidden"
                    onChange={(event) => {
                        void handle(event.target.files?.[0]);
                    }}
                />
            </div>
            {error && <div className="text-xs text-danger">{error}</div>}
        </div>
    );
};

export default UploadZone;
