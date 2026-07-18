import type { RuntimeInfo } from "../api/runtime";

type ExecutorsPageProps = { executors: RuntimeInfo[] };

function formatBytes(bytes: bigint): string {
  const gigabytes = Number(bytes) / 1_000_000_000;
  return `${gigabytes.toFixed(2)} GB`;
}

export function ExecutorsPage({ executors }: ExecutorsPageProps) {
  return (
    <section className="dashboard-page" data-testid="executors-page">
      <article className="executor-table-panel panel">
        <div className="table-scroll">
          <table className="executor-table">
            <thead>
              <tr>
                <th>Executor</th>
                <th>Status</th>
                <th>Model</th>
                <th>Runtime epoch</th>
                <th>Memory available</th>
                <th>KV cache</th>
                <th>Tensor parallel</th>
              </tr>
            </thead>
            <tbody>
              {executors.map((executor) => (
                <tr key={executor.executorId}>
                  <td>
                    <div className="executor-identity">
                      <span className="executor-mark">E</span>
                      <div>
                        <strong>{executor.executorId}</strong>
                        <small>
                          {executor.modelType.toUpperCase()} · {executor.deviceType.toUpperCase()} · {executor.numKvBlocks} BLOCKS
                        </small>
                      </div>
                    </div>
                  </td>
                  <td><span className="status-chip"><i />Ready</span></td>
                  <td className="mono-cell">{executor.modelId}</td>
                  <td className="mono-cell">{executor.runtimeEpoch}</td>
                  <td>{formatBytes(executor.availableMemoryBytes)} / {formatBytes(executor.totalMemoryBytes)}</td>
                  <td>{formatBytes(executor.kvCacheBytes)}</td>
                  <td>TP {executor.tensorParallelSize}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {executors.length === 0 ? <div className="empty-state">No executors reported by the Engine.</div> : null}
      </article>
    </section>
  );
}
