import React from 'react';
import { ExternalLink, Circle } from 'lucide-react';
import { Service } from '../types';

interface ServicesListProps {
  services: Service[];
}

export const ServicesList: React.FC<ServicesListProps> = ({ services }) => {
  if (services.length === 0) {
    return (
      <div className="text-sm text-slate-500 px-2 py-4 text-center">
        No services configured
      </div>
    );
  }

  return (
    <div className="space-y-1">
      {services.map((service) => (
        <div
          key={service.name}
          className="flex items-center justify-between px-2 py-1.5 rounded hover:bg-slate-700/50 transition-colors"
        >
          <div className="flex items-center gap-2">
            <Circle
              size={8}
              className={
                service.status === 'running'
                  ? 'fill-green-500 text-green-500'
                  : service.status === 'starting'
                  ? 'fill-yellow-500 text-yellow-500 animate-pulse'
                  : 'fill-slate-500 text-slate-500'
              }
            />
            <span className="text-sm text-slate-300">{service.name}</span>
            <span className="text-xs text-slate-500">:{service.port}</span>
          </div>

          {service.url && service.status === 'running' && (
            <a
              href={service.url}
              target="_blank"
              rel="noopener noreferrer"
              className="p-1 rounded hover:bg-slate-600 text-slate-400 hover:text-cyan-400 transition-colors"
            >
              <ExternalLink size={14} />
            </a>
          )}
        </div>
      ))}
    </div>
  );
};
