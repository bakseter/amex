// components/invoice-row.tsx
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
            className={`group mx-2 flex cursor-pointer items-center justify-between gap-2 rounded-lg px-3 py-2 transition-colors ${
                active ? 'bg-surface2' : 'hover:bg-surface2'
            }`}
        >
            <div className="min-w-0">
                <div className="truncate">{invoice.filename}</div>
                <div className="text-xs text-muted2">
                    {invoice.transaction_count} rows ·{' '}
                    {new Date(invoice.uploaded_at).toLocaleDateString('nb-NO')}
                </div>
            </div>
            <button
                onClick={(event) => {
                    event.stopPropagation();
                    void deleteInvoice(invoice.id);
                }}
                className="shrink-0 rounded-md px-1.5 text-muted opacity-0 transition group-hover:opacity-100 hover:text-danger focus-visible:opacity-100"
                title={`Delete ${invoice.filename}`}
            >
                ×
            </button>
        </div>
    );
};

export default InvoiceRow;
