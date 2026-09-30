const databaseName = "quantity-surveyor-drawings";

function openDrawings(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(databaseName, 1);
    request.onupgradeneeded = () => request.result.createObjectStore("drawings");
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

export async function saveDrawing(analysisId: string, file: File): Promise<void> {
  const database = await openDrawings();
  try {
    await new Promise<void>((resolve, reject) => {
      const transaction = database.transaction("drawings", "readwrite");
      transaction.objectStore("drawings").put(file, analysisId);
      transaction.oncomplete = () => resolve();
      transaction.onerror = () => reject(transaction.error);
    });
  } finally {
    database.close();
  }
}

export async function loadDrawing(analysisId: string): Promise<File | null> {
  const database = await openDrawings();
  try {
    return await new Promise<File | null>((resolve, reject) => {
      const request = database.transaction("drawings", "readonly").objectStore("drawings").get(analysisId);
      request.onsuccess = () => resolve(request.result instanceof File ? request.result : null);
      request.onerror = () => reject(request.error);
    });
  } finally {
    database.close();
  }
}