const ExportBar = ({ invoiceId }: { invoiceId: number }) => {
    const base = `http://localhost:8000/api/invoices/${String(invoiceId)}/export`;

    return (
        <div className="flex gap-2 px-3 py-2 border-t border-gray-100 shrink-0 text-xs">
            <a
                href={`${base}/transactions`}
                download
                className="text-blue-500 hover:underline"
            >
                ↓ Transactions CSV
            </a>
            <a
                href={`${base}/summary`}
                download
                className="text-blue-500 hover:underline"
            >
                ↓ Summary CSV
            </a>
        </div>
    );
};

export default ExportBar;
