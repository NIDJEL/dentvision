import { useEffect, useState } from "react";
import { getImageFile } from "../api/client";

type ProtectedImageProps = {
  token: string;
  imageID: number;
  alt: string;
};

export function ProtectedImage({ token, imageID, alt }: ProtectedImageProps) {
  const [src, setSrc] = useState("");
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let objectURL = "";
    let cancelled = false;

    getImageFile(token, imageID)
      .then((blob) => {
        if (cancelled) {
          return;
        }

        objectURL = URL.createObjectURL(blob);
        setSrc(objectURL);
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
        }
      });

    return () => {
      cancelled = true;
      if (objectURL) {
        URL.revokeObjectURL(objectURL);
      }
    };
  }, [imageID, token]);

  if (failed) {
    return <div className="image-placeholder">Image preview unavailable</div>;
  }

  if (!src) {
    return <div className="image-placeholder">Loading preview...</div>;
  }

  return <img className="image-preview" src={src} alt={alt} />;
}
