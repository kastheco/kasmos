import styles from "./Skeleton.module.css";

interface SkeletonCardProps {
  className?: string;
}

interface SkeletonRowProps {
  className?: string;
}

interface SkeletonBlockProps {
  className?: string;
}

interface SkeletonTextProps {
  lines?: number;
  className?: string;
}

type SkeletonProps =
  | ({ variant: "card" } & SkeletonCardProps)
  | ({ variant: "row" } & SkeletonRowProps)
  | ({ variant: "block" } & SkeletonBlockProps)
  | ({ variant: "text" } & SkeletonTextProps);

export default function Skeleton(props: SkeletonProps) {
  switch (props.variant) {
    case "card":
      return (
        <div
          className={[styles.skeleton, styles.card, props.className]
            .filter(Boolean)
            .join(" ")}
        />
      );

    case "row":
      return (
        <div
          className={[styles.skeleton, styles.row, props.className]
            .filter(Boolean)
            .join(" ")}
        />
      );

    case "block":
      return (
        <div
          className={[styles.skeleton, styles.block, props.className]
            .filter(Boolean)
            .join(" ")}
        />
      );

    case "text": {
      const count = props.lines ?? 3;
      return (
        <div
          className={[styles.textStack, props.className]
            .filter(Boolean)
            .join(" ")}
        >
          {Array.from({ length: count }, (_, i) => (
            <div
              key={i}
              className={[
                styles.skeleton,
                styles.textLine,
                i === count - 1 && count > 1 ? styles.textLineLast : "",
              ]
                .filter(Boolean)
                .join(" ")}
            />
          ))}
        </div>
      );
    }
  }
}
