import { useRef, useState } from 'react';

import { useUploadInvoiceMutation } from '@/api/transaction';

const UploadZone = ({ onUploaded }: { onUploaded: (id: number) => void }) => {
    const [upload, { isLoading }] = useUploadInvoiceMutation();
    const [dragging, setDragging] = useState(false);
    const ref = useRef<HTMLInputElement>(null);

    const handle = async (file: File) => {
        if (!file.name.endsWith('.pdf')) {
            return;
        }

        const res = await upload(file).unwrap();
        onUploaded(res.id);
    };

    return (
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

                const file = event.dataTransfer.files[0];
                if (file) {
                    handle(file);
                }
            }}
            className={`border-2 border-dashed rounded-lg px-8 py-6 text-center cursor-pointer transition-colors text-sm
                ${dragging ? 'border-blue-400 bg-blue-50' : 'border-gray-300 hover:border-gray-400 hover:bg-gray-50'}`}
        >
            <input
                ref={ref}
                type="file"
                accept=".pdf"
                className="hidden"
                onChange={(event) => {
                    const file = event.target.files?.[0];
                    if (file) {
                        handle(file);
                    }
                }}
            />
            {isLoading ? (
                <span className="text-gray-400">Parsing…</span>
            ) : (
                <span className="text-gray-500">
                    Drop a PDF invoice or{' '}
                    <span className="text-blue-500 underline">browse</span>
                </span>
            )}
        </div>
    );
};

export default UploadZone;
