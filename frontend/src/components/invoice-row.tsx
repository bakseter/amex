import { type Invoice, useDeleteInvoiceMutation } from '@/api/transaction';

const InvoiceRow = ({
    invoice,
    active,
    onSelect,
}: {
    invoice: Invoice;
    active: boolean;
    onSelect: () => void;
}) => {
    const [deleteInvoice] = useDeleteInvoiceMutation();

    return (
        <div
            onClick={onSelect}
            className={`flex items-center justify-between px-3 py-2 rounded-md cursor-pointer text-sm transition-colors
                ${active ? 'bg-blue-100 text-blue-800' : 'hover:bg-gray-100 text-gray-700'}`}
        >
            <div className="min-w-0">
                <div className="font-medium truncate">{invoice.filename}</div>
                <div className="text-xs text-gray-400">
                    {invoice.transaction_count} transactions ·{' '}
                    {new Date(invoice.uploaded_at).toLocaleDateString()}
                </div>
            </div>
            <button
                onClick={(event) => {
                    event.stopPropagation();
                    void deleteInvoice(invoice.id);
                }}
                className="ml-2 shrink-0 text-gray-300 hover:text-red-400 transition-colors text-lg leading-none"
                title="Delete"
            >
                ×
            </button>
        </div>
    );
};

export default InvoiceRow;
