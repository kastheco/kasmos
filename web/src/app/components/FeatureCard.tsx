import styles from "./FeatureCard.module.css";

interface FeatureCardProps {
  icon: string;
  title: string;
  description: string;
  tone: "iris" | "foam" | "gold" | "rose" | "pine";
}

const toneClasses = {
  iris: styles.toneIris,
  foam: styles.toneFoam,
  gold: styles.toneGold,
  rose: styles.toneRose,
  pine: styles.tonePine,
};

export default function FeatureCard({ icon, title, description, tone }: FeatureCardProps) {
  return (
    <div className={`${styles.card} ${toneClasses[tone]}`}>
      <span className={styles.icon}>{icon}</span>
      <h3 className={styles.title}>{title}</h3>
      <p className={styles.description}>{description}</p>
    </div>
  );
}
